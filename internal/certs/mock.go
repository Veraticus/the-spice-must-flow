package certs

import (
	"crypto/tls"
	"errors"
)

// MockManager is a mock implementation of Manager for testing.
type MockManager struct {
	Certificate     tls.Certificate
	GetError        error
	ExistsError     error
	GetCallCount    int
	ExistsCallCount int
	Exists          bool
}

// GetOrCreateCertificate returns the configured certificate or error.
func (m *MockManager) GetOrCreateCertificate() (tls.Certificate, error) {
	m.GetCallCount++
	if m.GetError != nil {
		return tls.Certificate{}, m.GetError
	}
	return m.Certificate, nil
}

// CertificateExists returns the configured existence state or error.
func (m *MockManager) CertificateExists() (bool, error) {
	m.ExistsCallCount++
	if m.ExistsError != nil {
		return false, m.ExistsError
	}
	return m.Exists, nil
}

// NewMockManager creates a new mock manager with a test certificate.
func NewMockManager() *MockManager {
	return &MockManager{
		Certificate: tls.Certificate{
			Certificate: [][]byte{{1, 2, 3}}, // Dummy certificate data
		},
		Exists: true,
	}
}

// NewFailingMockManager creates a mock manager that returns errors.
func NewFailingMockManager(errMsg string) *MockManager {
	err := errors.New(errMsg)
	return &MockManager{
		GetError:    err,
		ExistsError: err,
	}
}
