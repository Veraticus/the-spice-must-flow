// Package certs provides TLS certificate generation and management for local HTTPS servers.
package certs

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Manager defines the interface for certificate operations.

// FileManager implements Manager using the filesystem.
type FileManager struct {
	certDir  string
	certFile string
	keyFile  string
}

// NewFileManager creates a new FileManager with the specified certificate directory.
func NewFileManager(certDir string) *FileManager {
	return &FileManager{
		certDir:  certDir,
		certFile: filepath.Join(certDir, "localhost.crt"),
		keyFile:  filepath.Join(certDir, "localhost.key"),
	}
}

// GetOrCreateCertificate returns an existing certificate or creates a new one.
func (m *FileManager) GetOrCreateCertificate() (tls.Certificate, error) {
	// Check if certificate exists and is valid
	if exists, err := m.CertificateExists(); err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to check certificate existence: %w", err)
	} else if exists {
		cert, err := tls.LoadX509KeyPair(m.certFile, m.keyFile)
		if err != nil {
			// Certificate files exist but are invalid, remove and regenerate
			if err := m.removeCertificates(); err != nil {
				return tls.Certificate{}, fmt.Errorf("failed to remove invalid certificate files: %w", err)
			}
			// Fall through to generate new certificate
		} else {
			// Verify the certificate is still valid
			if err := m.verifyCertificate(cert); err != nil {
				// Certificate is invalid, regenerate
				if err := m.removeCertificates(); err != nil {
					return tls.Certificate{}, fmt.Errorf("failed to remove invalid certificate: %w", err)
				}
				// Fall through to generate new certificate
			} else {
				// Certificate is valid, return it
				return cert, nil
			}
		}
	}

	// Generate new certificate
	return m.generateCertificate()
}

// CertificateExists checks if both certificate and key files exist.
func (m *FileManager) CertificateExists() (bool, error) {
	if _, err := os.Stat(m.certFile); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check certificate file: %w", err)
	}

	if _, err := os.Stat(m.keyFile); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check key file: %w", err)
	}

	return true, nil
}

// generateCertificate creates a new self-signed certificate for localhost.
func (m *FileManager) generateCertificate() (tls.Certificate, error) {
	// Create certificate directory if it doesn't exist
	if err := os.MkdirAll(m.certDir, 0700); err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to create certificate directory: %w", err)
	}

	// Generate RSA private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Spice Financial Manager"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour), // Valid for 1 year
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses: []net.IP{
			net.IPv4(127, 0, 0, 1),
			net.IPv6loopback,
		},
		DNSNames: []string{
			"localhost",
			"*.localhost",
		},
	}

	// Generate certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to create certificate: %w", err)
	}

	// Save certificate to file
	certOut, err := os.OpenFile(m.certFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to open certificate file for writing: %w", err)
	}
	defer func() { _ = certOut.Close() }()

	if encodeErr := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); encodeErr != nil {
		return tls.Certificate{}, fmt.Errorf("failed to write certificate: %w", encodeErr)
	}

	// Save private key to file
	keyOut, err := os.OpenFile(m.keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to open key file for writing: %w", err)
	}
	defer func() { _ = keyOut.Close() }()

	privKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	}
	if err := pem.Encode(keyOut, privKeyPEM); err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to write private key: %w", err)
	}

	// Load the certificate we just created
	return tls.LoadX509KeyPair(m.certFile, m.keyFile)
}

// verifyCertificate checks if a certificate is still valid.
func (m *FileManager) verifyCertificate(cert tls.Certificate) error {
	if len(cert.Certificate) == 0 {
		return fmt.Errorf("no certificates found")
	}

	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Check if certificate has expired
	now := time.Now()
	if now.Before(x509Cert.NotBefore) {
		return fmt.Errorf("certificate not yet valid")
	}
	if now.After(x509Cert.NotAfter) {
		return fmt.Errorf("certificate has expired")
	}

	// Verify it's valid for localhost
	if err := x509Cert.VerifyHostname("localhost"); err != nil {
		return fmt.Errorf("certificate not valid for localhost: %w", err)
	}

	return nil
}

// removeCertificates removes existing certificate files.
func (m *FileManager) removeCertificates() error {
	if err := os.Remove(m.certFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove certificate file: %w", err)
	}
	if err := os.Remove(m.keyFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove key file: %w", err)
	}
	return nil
}
