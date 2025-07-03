package sheets

import (
	"context"
	"sync"

	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
)

// MockWriter is a mock implementation of ReportWriter for testing.
type MockWriter struct {
	WriteFunc           func(ctx context.Context, classifications []model.Classification, summary *service.ReportSummary, categories []model.Category) error
	LastSummary         *service.ReportSummary
	WriteCalls          []WriteCall
	LastClassifications []model.Classification
	WriteCallCount      int
	mu                  sync.Mutex
}

// WriteCall represents a single call to Write.
type WriteCall struct {
	Error           error
	Summary         *service.ReportSummary
	Classifications []model.Classification
}

// NewMockWriter creates a new mock writer.
func NewMockWriter() *MockWriter {
	return &MockWriter{
		WriteCalls: make([]WriteCall, 0),
	}
}

// Write implements the ReportWriter interface.
func (m *MockWriter) Write(ctx context.Context, classifications []model.Classification, summary *service.ReportSummary, categories []model.Category) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.WriteCallCount++
	m.LastClassifications = classifications
	m.LastSummary = summary

	var err error
	if m.WriteFunc != nil {
		err = m.WriteFunc(ctx, classifications, summary, categories)
	}

	m.WriteCalls = append(m.WriteCalls, WriteCall{
		Classifications: classifications,
		Summary:         summary,
		Error:           err,
	})

	return err
}

// Reset clears all recorded calls.
func (m *MockWriter) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.WriteCallCount = 0
	m.WriteCalls = make([]WriteCall, 0)
	m.LastClassifications = nil
	m.LastSummary = nil
}

// GetWriteCalls returns a copy of all write calls.
func (m *MockWriter) GetWriteCalls() []WriteCall {
	m.mu.Lock()
	defer m.mu.Unlock()

	calls := make([]WriteCall, len(m.WriteCalls))
	copy(calls, m.WriteCalls)
	return calls
}

// AssertWriteCalled verifies that Write was called with expected parameters.
func (m *MockWriter) AssertWriteCalled(t interface{ Fatalf(string, ...any) }, expectedCalls int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.WriteCallCount != expectedCalls {
		t.Fatalf("expected Write to be called %d times, but was called %d times", expectedCalls, m.WriteCallCount)
	}
}

// SetWriteError configures the mock to return an error on the next Write call.
func (m *MockWriter) SetWriteError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.WriteFunc = func(_ context.Context, _ []model.Classification, _ *service.ReportSummary, _ []model.Category) error {
		return err
	}
}
