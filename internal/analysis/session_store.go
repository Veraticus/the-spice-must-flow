package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// MemorySessionStore implements SessionStore and ReportStore using in-memory storage.
// This is a simple implementation suitable for single-instance applications.
// For production use, implement a persistent store using the storage package.
type MemorySessionStore struct {
	sessions        map[string]*Session
	reports         map[string]*Report
	stopCh          chan struct{}
	cleanupInterval time.Duration
	maxAge          time.Duration
	mu              sync.RWMutex
}

// NewMemorySessionStore creates a new in-memory session store.
func NewMemorySessionStore() *MemorySessionStore {
	store := &MemorySessionStore{
		sessions:        make(map[string]*Session),
		reports:         make(map[string]*Report),
		cleanupInterval: 1 * time.Hour,
		maxAge:          24 * time.Hour,
		stopCh:          make(chan struct{}),
	}

	// Start cleanup goroutine
	go store.cleanupLoop()

	return store
}

// Create creates a new analysis session.
func (s *MemorySessionStore) Create(ctx context.Context, session *Session) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	if session.ID == "" {
		return fmt.Errorf("session ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[session.ID]; exists {
		return fmt.Errorf("session already exists: %s", session.ID)
	}

	// Create a copy to avoid external modifications
	sessionCopy := *session
	s.sessions[session.ID] = &sessionCopy

	return nil
}

// Get retrieves an analysis session by ID.
func (s *MemorySessionStore) Get(ctx context.Context, sessionID string) (*Session, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	if sessionID == "" {
		return nil, fmt.Errorf("session ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	// Return a copy to avoid external modifications
	sessionCopy := *session
	return &sessionCopy, nil
}

// Update updates an existing analysis session.
func (s *MemorySessionStore) Update(ctx context.Context, session *Session) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	if session == nil {
		return fmt.Errorf("session cannot be nil")
	}

	if session.ID == "" {
		return fmt.Errorf("session ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[session.ID]; !exists {
		return fmt.Errorf("session not found: %s", session.ID)
	}

	// Update with a copy
	sessionCopy := *session
	s.sessions[session.ID] = &sessionCopy

	return nil
}

// SaveReport stores an analysis report.
func (s *MemorySessionStore) SaveReport(ctx context.Context, report *Report) error {
	if err := validateContext(ctx); err != nil {
		return err
	}

	if report == nil {
		return fmt.Errorf("report cannot be nil")
	}

	if err := report.Validate(); err != nil {
		return fmt.Errorf("invalid report: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.reports[report.ID]; exists {
		return fmt.Errorf("report already exists: %s", report.ID)
	}

	// Create a deep copy of the report
	reportCopy := s.deepCopyReport(report)
	s.reports[report.ID] = reportCopy

	// Update the associated session with the report ID
	if session, exists := s.sessions[report.SessionID]; exists {
		session.ReportID = &report.ID
	}

	return nil
}

// GetReport retrieves a report by ID.
func (s *MemorySessionStore) GetReport(ctx context.Context, reportID string) (*Report, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	if reportID == "" {
		return nil, fmt.Errorf("report ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	report, exists := s.reports[reportID]
	if !exists {
		return nil, fmt.Errorf("report not found: %s", reportID)
	}

	// Return a deep copy to avoid external modifications
	return s.deepCopyReport(report), nil
}

// GetActiveSessions returns all active (non-completed, non-failed) sessions.
func (s *MemorySessionStore) GetActiveSessions(ctx context.Context) ([]*Session, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var activeSessions []*Session
	for _, session := range s.sessions {
		if session.Status != StatusCompleted && session.Status != StatusFailed {
			sessionCopy := *session
			activeSessions = append(activeSessions, &sessionCopy)
		}
	}

	return activeSessions, nil
}

// GetReportsByDateRange returns all reports within the specified date range.
func (s *MemorySessionStore) GetReportsByDateRange(ctx context.Context, start, end time.Time) ([]*Report, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var reports []*Report
	for _, report := range s.reports {
		if !report.PeriodStart.Before(start) && !report.PeriodEnd.After(end) {
			reports = append(reports, s.deepCopyReport(report))
		}
	}

	return reports, nil
}

// cleanupLoop periodically removes old completed/failed sessions.
func (s *MemorySessionStore) cleanupLoop() {
	ticker := time.NewTicker(s.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.cleanup()
		case <-s.stopCh:
			return
		}
	}
}

// cleanup removes old sessions and their associated reports.
func (s *MemorySessionStore) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-s.maxAge)

	// Find sessions to remove
	var sessionsToRemove []string
	for id, session := range s.sessions {
		if session.Status == StatusCompleted || session.Status == StatusFailed {
			if session.LastAttempt.Before(cutoff) {
				sessionsToRemove = append(sessionsToRemove, id)
			}
		}
	}

	// Remove old sessions and their reports
	for _, sessionID := range sessionsToRemove {
		session := s.sessions[sessionID]

		// Remove associated report if exists
		if session.ReportID != nil {
			delete(s.reports, *session.ReportID)
		}

		// Remove session
		delete(s.sessions, sessionID)
	}
}

// deepCopyReport creates a deep copy of a report.
func (s *MemorySessionStore) deepCopyReport(report *Report) *Report {
	if report == nil {
		return nil
	}

	// Use JSON marshaling for deep copy (simple but effective)
	data, err := json.Marshal(report)
	if err != nil {
		// This shouldn't happen with valid reports
		panic(fmt.Sprintf("failed to marshal report for deep copy: %v", err))
	}

	var reportCopy Report
	if err := json.Unmarshal(data, &reportCopy); err != nil {
		// This shouldn't happen
		panic(fmt.Sprintf("failed to unmarshal report for deep copy: %v", err))
	}

	return &reportCopy
}

// validateContext ensures the context is valid.
func validateContext(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("context cannot be nil")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// Stop gracefully shuts down the session store.
func (s *MemorySessionStore) Stop() {
	close(s.stopCh)
}
