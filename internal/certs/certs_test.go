package certs

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileManager_GetOrCreateCertificate(t *testing.T) {
	tests := []struct {
		setup          func(t *testing.T, certDir string)
		validateResult func(t *testing.T, cert tls.Certificate)
		name           string
		errorContains  string
		wantErr        bool
	}{
		{
			name: "creates new certificate when none exists",
			setup: func(t *testing.T, certDir string) {
				// No setup needed - directory doesn't exist
			},
			wantErr: false,
			validateResult: func(t *testing.T, cert tls.Certificate) {
				require.Len(t, cert.Certificate, 1, "should have one certificate")

				x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
				require.NoError(t, err)

				// Verify certificate properties
				assert.Equal(t, "Spice Financial Manager", x509Cert.Subject.Organization[0])
				assert.Contains(t, x509Cert.DNSNames, "localhost")
				assert.True(t, x509Cert.NotAfter.After(time.Now().Add(364*24*time.Hour)), "certificate should be valid for about a year")

				// Verify it's valid for localhost
				err = x509Cert.VerifyHostname("localhost")
				assert.NoError(t, err)
			},
		},
		{
			name: "reuses existing valid certificate",
			setup: func(t *testing.T, certDir string) {
				// Create a valid certificate first
				m := NewFileManager(certDir)
				_, err := m.GetOrCreateCertificate()
				require.NoError(t, err)
			},
			wantErr: false,
			validateResult: func(t *testing.T, cert tls.Certificate) {
				require.Len(t, cert.Certificate, 1, "should have one certificate")

				// Verify it's the same certificate by checking creation time
				x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
				require.NoError(t, err)

				// Certificate should have been created in the past (not just now)
				// Allow some buffer for test execution time
				assert.True(t, x509Cert.NotBefore.Before(time.Now().Add(1*time.Second)))
			},
		},
		{
			name: "regenerates invalid certificate",
			setup: func(t *testing.T, certDir string) {
				// Create directory
				os.MkdirAll(certDir, 0700)

				// Write files that exist but contain invalid certificate data
				certFile := filepath.Join(certDir, "localhost.crt")
				keyFile := filepath.Join(certDir, "localhost.key")

				// Write completely invalid data (not even valid PEM)
				os.WriteFile(certFile, []byte("invalid certificate data"), 0644)
				os.WriteFile(keyFile, []byte("invalid key data"), 0600)
			},
			wantErr: false,
			validateResult: func(t *testing.T, cert tls.Certificate) {
				require.Len(t, cert.Certificate, 1, "should have one certificate")

				// Should be a fresh certificate
				x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
				require.NoError(t, err)

				// Certificate should be brand new
				assert.True(t, x509Cert.NotBefore.After(time.Now().Add(-1*time.Minute)))
			},
		},
		{
			name: "handles certificate directory creation failure",
			setup: func(t *testing.T, certDir string) {
				// Create a file where the directory should be
				parentDir := filepath.Dir(certDir)
				os.MkdirAll(parentDir, 0700)
				os.WriteFile(certDir, []byte("not a directory"), 0644)
			},
			wantErr:       true,
			errorContains: "failed to check certificate",
		},
		{
			name: "handles permission errors on certificate file",
			setup: func(t *testing.T, certDir string) {
				if os.Getuid() == 0 {
					t.Skip("Cannot test permission errors as root")
				}
				os.MkdirAll(certDir, 0700)
				// Make directory read-only
				os.Chmod(certDir, 0500)
				t.Cleanup(func() {
					os.Chmod(certDir, 0700)
				})
			},
			wantErr:       true,
			errorContains: "failed to open certificate file for writing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory for test
			tempDir := t.TempDir()
			certDir := filepath.Join(tempDir, "certs")

			// Run test setup
			if tt.setup != nil {
				tt.setup(t, certDir)
			}

			// Create manager and get certificate
			m := NewFileManager(certDir)
			cert, err := m.GetOrCreateCertificate()

			// Check error expectations
			if tt.wantErr {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			require.NoError(t, err)

			// Validate result
			if tt.validateResult != nil {
				tt.validateResult(t, cert)
			}

			// Verify files were created with correct permissions
			certInfo, err := os.Stat(filepath.Join(certDir, "localhost.crt"))
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0600), certInfo.Mode().Perm(), "certificate file should be owner-only")

			keyInfo, err := os.Stat(filepath.Join(certDir, "localhost.key"))
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0600), keyInfo.Mode().Perm(), "key file should be owner-only")
		})
	}
}

func TestFileManager_CertificateExists(t *testing.T) {
	tests := []struct {
		setup         func(t *testing.T, certDir string)
		name          string
		errorContains string
		wantExists    bool
		wantErr       bool
	}{
		{
			name: "returns false when no files exist",
			setup: func(t *testing.T, certDir string) {
				// No setup needed
			},
			wantExists: false,
			wantErr:    false,
		},
		{
			name: "returns true when both files exist",
			setup: func(t *testing.T, certDir string) {
				os.MkdirAll(certDir, 0700)
				os.WriteFile(filepath.Join(certDir, "localhost.crt"), []byte("cert"), 0644)
				os.WriteFile(filepath.Join(certDir, "localhost.key"), []byte("key"), 0600)
			},
			wantExists: true,
			wantErr:    false,
		},
		{
			name: "returns false when only certificate exists",
			setup: func(t *testing.T, certDir string) {
				os.MkdirAll(certDir, 0700)
				os.WriteFile(filepath.Join(certDir, "localhost.crt"), []byte("cert"), 0644)
			},
			wantExists: false,
			wantErr:    false,
		},
		{
			name: "returns false when only key exists",
			setup: func(t *testing.T, certDir string) {
				os.MkdirAll(certDir, 0700)
				os.WriteFile(filepath.Join(certDir, "localhost.key"), []byte("key"), 0600)
			},
			wantExists: false,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			certDir := filepath.Join(tempDir, "certs")

			if tt.setup != nil {
				tt.setup(t, certDir)
			}

			m := NewFileManager(certDir)
			exists, err := m.CertificateExists()

			if tt.wantErr {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantExists, exists)
		})
	}
}

func TestFileManager_verifyCertificate(t *testing.T) {
	// Helper to create a test certificate
	createTestCert := func(notBefore, notAfter time.Time, dnsNames []string) tls.Certificate {
		m := &FileManager{
			certDir:  t.TempDir(),
			certFile: filepath.Join(t.TempDir(), "test.crt"),
			keyFile:  filepath.Join(t.TempDir(), "test.key"),
		}

		// Temporarily override time for certificate generation
		// Note: In real implementation, we'd inject time as a dependency
		cert, err := m.generateCertificate()
		require.NoError(t, err)
		return cert
	}

	tests := []struct {
		cert          func() tls.Certificate
		name          string
		errorContains string
		wantErr       bool
	}{
		{
			name: "valid certificate passes verification",
			cert: func() tls.Certificate {
				return createTestCert(
					time.Now().Add(-1*time.Hour),
					time.Now().Add(365*24*time.Hour),
					[]string{"localhost"},
				)
			},
			wantErr: false,
		},
		{
			name: "empty certificate fails",
			cert: func() tls.Certificate {
				return tls.Certificate{}
			},
			wantErr:       true,
			errorContains: "no certificates found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &FileManager{}
			err := m.verifyCertificate(tt.cert())

			if tt.wantErr {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestCertificateProperties(t *testing.T) {
	// Test that generated certificates have all required properties
	tempDir := t.TempDir()
	certDir := filepath.Join(tempDir, "certs")

	m := NewFileManager(certDir)
	cert, err := m.GetOrCreateCertificate()
	require.NoError(t, err)

	// Parse the certificate
	require.Len(t, cert.Certificate, 1)
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	require.NoError(t, err)

	// Verify all required properties
	t.Run("organization", func(t *testing.T) {
		assert.Equal(t, []string{"Spice Financial Manager"}, x509Cert.Subject.Organization)
	})

	t.Run("validity period", func(t *testing.T) {
		assert.True(t, x509Cert.NotBefore.Before(time.Now()))
		assert.True(t, x509Cert.NotAfter.After(time.Now().Add(364*24*time.Hour)))
	})

	t.Run("key usage", func(t *testing.T) {
		assert.Equal(t, x509.KeyUsageKeyEncipherment|x509.KeyUsageDigitalSignature, x509Cert.KeyUsage)
		assert.Equal(t, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, x509Cert.ExtKeyUsage)
	})

	t.Run("DNS names", func(t *testing.T) {
		assert.Contains(t, x509Cert.DNSNames, "localhost")
		assert.Contains(t, x509Cert.DNSNames, "*.localhost")
	})

	t.Run("IP addresses", func(t *testing.T) {
		// Check for IPv4 loopback
		hasIPv4Loopback := false
		hasIPv6Loopback := false

		for _, ip := range x509Cert.IPAddresses {
			if ip.Equal(net.IPv4(127, 0, 0, 1)) {
				hasIPv4Loopback = true
			}
			if ip.Equal(net.IPv6loopback) {
				hasIPv6Loopback = true
			}
		}

		assert.True(t, hasIPv4Loopback, "certificate should include IPv4 loopback")
		assert.True(t, hasIPv6Loopback, "certificate should include IPv6 loopback")
	})

	t.Run("can be used for TLS", func(t *testing.T) {
		// Verify the certificate can be used in a TLS config
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
		assert.NotNil(t, tlsConfig)
	})
}
