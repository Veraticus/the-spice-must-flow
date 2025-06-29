package analysis

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemorySessionStore_SessionOperations(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Stop()
	ctx := context.Background()

	// Create test session
	session := &Session{
		ID:          "session-123",
		StartedAt:   time.Now(),
		LastAttempt: time.Now(),
		Status:      StatusPending,
		Attempts:    0,
	}

	t.Run("Create", func(t *testing.T) {
		// Test successful creation
		err := store.Create(ctx, session)
		require.NoError(t, err)

		// Test duplicate creation
		err = store.Create(ctx, session)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")

		// Test nil session
		err = store.Create(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")

		// Test empty ID
		emptySession := &Session{StartedAt: time.Now()}
		err = store.Create(ctx, emptySession)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ID is required")

		// Test nil context
		err = store.Create(nil, session)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context cannot be nil")
	})

	t.Run("Get", func(t *testing.T) {
		// Test successful retrieval
		retrieved, err := store.Get(ctx, session.ID)
		require.NoError(t, err)
		assert.Equal(t, session.ID, retrieved.ID)
		assert.Equal(t, session.Status, retrieved.Status)

		// Test non-existent session
		_, err = store.Get(ctx, "non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")

		// Test empty ID
		_, err = store.Get(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ID is required")

		// Test nil context
		_, err = store.Get(nil, session.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context cannot be nil")
	})

	t.Run("Update", func(t *testing.T) {
		// Update session
		session.Status = StatusInProgress
		session.Attempts = 1
		err := store.Update(ctx, session)
		require.NoError(t, err)

		// Verify update
		retrieved, err := store.Get(ctx, session.ID)
		require.NoError(t, err)
		assert.Equal(t, StatusInProgress, retrieved.Status)
		assert.Equal(t, 1, retrieved.Attempts)

		// Test updating non-existent session
		nonExistent := &Session{ID: "non-existent"}
		err = store.Update(ctx, nonExistent)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")

		// Test nil session
		err = store.Update(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")

		// Test empty ID
		emptySession := &Session{}
		err = store.Update(ctx, emptySession)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ID is required")
	})

	t.Run("SessionIsolation", func(t *testing.T) {
		// Verify that modifications to retrieved session don't affect stored session
		retrieved, err := store.Get(ctx, session.ID)
		require.NoError(t, err)

		retrieved.Status = StatusFailed

		// Get again and verify original status preserved
		retrieved2, err := store.Get(ctx, session.ID)
		require.NoError(t, err)
		assert.Equal(t, StatusInProgress, retrieved2.Status)
	})
}

func TestMemorySessionStore_ReportOperations(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Stop()
	ctx := context.Background()

	// Create session first
	session := &Session{
		ID:          "session-456",
		StartedAt:   time.Now(),
		LastAttempt: time.Now(),
		Status:      StatusInProgress,
	}
	err := store.Create(ctx, session)
	require.NoError(t, err)

	// Create test report
	now := time.Now()
	report := &Report{
		ID:             "report-123",
		SessionID:      session.ID,
		GeneratedAt:    now,
		PeriodStart:    now.AddDate(0, -1, 0),
		PeriodEnd:      now,
		CoherenceScore: 0.85,
		Issues: []Issue{
			{
				ID:             "issue-1",
				Type:           IssueTypeMiscategorized,
				Severity:       SeverityHigh,
				Description:    "Test issue",
				TransactionIDs: []string{"txn-1"},
				AffectedCount:  1,
				Confidence:     0.9,
			},
		},
		SuggestedPatterns: []SuggestedPattern{},
		Insights:          []string{"Test insight"},
		CategorySummary: map[string]CategoryStat{
			"cat-1": {
				CategoryID:       "cat-1",
				CategoryName:     "Test Category",
				TransactionCount: 10,
				TotalAmount:      1000.0,
				Consistency:      0.9,
				Issues:           1,
			},
		},
	}

	t.Run("SaveReport", func(t *testing.T) {
		// Test successful save
		err := store.SaveReport(ctx, report)
		require.NoError(t, err)

		// Verify session was updated with report ID
		updatedSession, err := store.Get(ctx, session.ID)
		require.NoError(t, err)
		assert.NotNil(t, updatedSession.ReportID)
		assert.Equal(t, report.ID, *updatedSession.ReportID)

		// Test duplicate save
		err = store.SaveReport(ctx, report)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")

		// Test nil report
		err = store.SaveReport(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")

		// Test invalid report
		invalidReport := &Report{
			ID:        "invalid",
			SessionID: "", // Missing required field
		}
		err = store.SaveReport(ctx, invalidReport)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid report")
	})

	t.Run("GetReport", func(t *testing.T) {
		// Test successful retrieval
		retrieved, err := store.GetReport(ctx, report.ID)
		require.NoError(t, err)
		assert.Equal(t, report.ID, retrieved.ID)
		assert.Equal(t, report.SessionID, retrieved.SessionID)
		assert.Equal(t, report.CoherenceScore, retrieved.CoherenceScore)
		assert.Len(t, retrieved.Issues, 1)
		assert.Len(t, retrieved.Insights, 1)
		assert.Len(t, retrieved.CategorySummary, 1)

		// Test non-existent report
		_, err = store.GetReport(ctx, "non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")

		// Test empty ID
		_, err = store.GetReport(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ID is required")
	})

	t.Run("ReportIsolation", func(t *testing.T) {
		// Verify that modifications to retrieved report don't affect stored report
		retrieved, err := store.GetReport(ctx, report.ID)
		require.NoError(t, err)

		retrieved.CoherenceScore = 0.5
		retrieved.Issues = append(retrieved.Issues, Issue{ID: "new-issue"})

		// Get again and verify original values preserved
		retrieved2, err := store.GetReport(ctx, report.ID)
		require.NoError(t, err)
		assert.Equal(t, 0.85, retrieved2.CoherenceScore)
		assert.Len(t, retrieved2.Issues, 1)
	})
}

func TestMemorySessionStore_GetActiveSessions(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Stop()
	ctx := context.Background()

	// Create various sessions
	sessions := []*Session{
		{ID: "pending", Status: StatusPending, StartedAt: time.Now(), LastAttempt: time.Now()},
		{ID: "in-progress", Status: StatusInProgress, StartedAt: time.Now(), LastAttempt: time.Now()},
		{ID: "validating", Status: StatusValidating, StartedAt: time.Now(), LastAttempt: time.Now()},
		{ID: "completed", Status: StatusCompleted, StartedAt: time.Now(), LastAttempt: time.Now()},
		{ID: "failed", Status: StatusFailed, StartedAt: time.Now(), LastAttempt: time.Now()},
	}

	for _, s := range sessions {
		err := store.Create(ctx, s)
		require.NoError(t, err)
	}

	// Get active sessions
	active, err := store.GetActiveSessions(ctx)
	require.NoError(t, err)

	// Should have 3 active sessions (pending, in-progress, validating)
	assert.Len(t, active, 3)

	// Verify correct sessions returned
	activeIDs := make(map[string]bool)
	for _, s := range active {
		activeIDs[s.ID] = true
	}

	assert.True(t, activeIDs["pending"])
	assert.True(t, activeIDs["in-progress"])
	assert.True(t, activeIDs["validating"])
	assert.False(t, activeIDs["completed"])
	assert.False(t, activeIDs["failed"])
}

func TestMemorySessionStore_GetReportsByDateRange(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Stop()
	ctx := context.Background()

	now := time.Now()

	// Create reports with different date ranges
	reports := []*Report{
		{
			ID:             "report-1",
			SessionID:      "session-1",
			GeneratedAt:    now,
			PeriodStart:    now.AddDate(0, -3, 0),
			PeriodEnd:      now.AddDate(0, -2, 0),
			CoherenceScore: 0.8,
		},
		{
			ID:             "report-2",
			SessionID:      "session-2",
			GeneratedAt:    now,
			PeriodStart:    now.AddDate(0, -2, 0),
			PeriodEnd:      now.AddDate(0, -1, 0),
			CoherenceScore: 0.85,
		},
		{
			ID:             "report-3",
			SessionID:      "session-3",
			GeneratedAt:    now,
			PeriodStart:    now.AddDate(0, -1, 0),
			PeriodEnd:      now,
			CoherenceScore: 0.9,
		},
	}

	// Create dummy sessions for the reports
	for i, report := range reports {
		session := &Session{
			ID:          report.SessionID,
			StartedAt:   now,
			LastAttempt: now,
			Status:      StatusCompleted,
		}
		err := store.Create(ctx, session)
		require.NoError(t, err, "Failed to create session %d", i)

		err = store.SaveReport(ctx, report)
		require.NoError(t, err, "Failed to save report %d", i)
	}

	tests := []struct {
		start     time.Time
		end       time.Time
		name      string
		wantIDs   []string
		wantCount int
	}{
		{
			name:      "all reports",
			start:     now.AddDate(0, -4, 0),
			end:       now.AddDate(0, 0, 1),
			wantCount: 3,
			wantIDs:   []string{"report-1", "report-2", "report-3"},
		},
		{
			name:      "last two months",
			start:     now.AddDate(0, -2, 0),
			end:       now,
			wantCount: 2,
			wantIDs:   []string{"report-2", "report-3"},
		},
		{
			name:      "last month only",
			start:     now.AddDate(0, -1, 0),
			end:       now,
			wantCount: 1,
			wantIDs:   []string{"report-3"},
		},
		{
			name:      "no reports in range",
			start:     now.AddDate(0, -6, 0),
			end:       now.AddDate(0, -5, 0),
			wantCount: 0,
			wantIDs:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := store.GetReportsByDateRange(ctx, tt.start, tt.end)
			require.NoError(t, err)
			assert.Len(t, result, tt.wantCount)

			// Verify correct reports returned
			resultIDs := make(map[string]bool)
			for _, r := range result {
				resultIDs[r.ID] = true
			}

			for _, wantID := range tt.wantIDs {
				assert.True(t, resultIDs[wantID], "Expected report %s not found", wantID)
			}
		})
	}
}

func TestMemorySessionStore_Cleanup(t *testing.T) {
	// Create store with short cleanup intervals for testing
	store := &MemorySessionStore{
		sessions:        make(map[string]*Session),
		reports:         make(map[string]*Report),
		cleanupInterval: 100 * time.Millisecond,
		maxAge:          200 * time.Millisecond,
		stopCh:          make(chan struct{}),
	}
	defer store.Stop()

	ctx := context.Background()
	now := time.Now()

	// Create old and new sessions
	oldSession := &Session{
		ID:          "old-session",
		StartedAt:   now.Add(-1 * time.Hour),
		LastAttempt: now.Add(-1 * time.Hour),
		Status:      StatusCompleted,
		ReportID:    stringPtr("old-report"),
	}

	newSession := &Session{
		ID:          "new-session",
		StartedAt:   now,
		LastAttempt: now,
		Status:      StatusCompleted,
	}

	activeSession := &Session{
		ID:          "active-session",
		StartedAt:   now.Add(-1 * time.Hour),
		LastAttempt: now.Add(-1 * time.Hour),
		Status:      StatusInProgress, // Active status
	}

	// Create sessions
	err := store.Create(ctx, oldSession)
	require.NoError(t, err)
	err = store.Create(ctx, newSession)
	require.NoError(t, err)
	err = store.Create(ctx, activeSession)
	require.NoError(t, err)

	// Create report for old session
	oldReport := &Report{
		ID:             "old-report",
		SessionID:      oldSession.ID,
		GeneratedAt:    now.Add(-1 * time.Hour),
		PeriodStart:    now.AddDate(0, -1, 0),
		PeriodEnd:      now,
		CoherenceScore: 0.8,
	}
	err = store.SaveReport(ctx, oldReport)
	require.NoError(t, err)

	// Verify all exist initially
	_, err = store.Get(ctx, oldSession.ID)
	assert.NoError(t, err)
	_, err = store.Get(ctx, newSession.ID)
	assert.NoError(t, err)
	_, err = store.Get(ctx, activeSession.ID)
	assert.NoError(t, err)
	_, err = store.GetReport(ctx, oldReport.ID)
	assert.NoError(t, err)

	// Manually trigger cleanup with adjusted time
	store.maxAge = 30 * time.Minute // Make only sessions older than 30 minutes eligible for cleanup
	store.cleanup()

	// Verify old session and report are removed
	_, err = store.Get(ctx, oldSession.ID)
	assert.Error(t, err, "Old session should be removed")
	_, err = store.GetReport(ctx, oldReport.ID)
	assert.Error(t, err, "Old report should be removed")

	// Verify new and active sessions remain
	_, err = store.Get(ctx, newSession.ID)
	assert.NoError(t, err, "New session should remain")
	_, err = store.Get(ctx, activeSession.ID)
	assert.NoError(t, err, "Active session should remain")
}

func TestMemorySessionStore_ConcurrentAccess(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Stop()
	ctx := context.Background()

	// Test concurrent session operations
	t.Run("ConcurrentSessions", func(t *testing.T) {
		done := make(chan bool, 3)

		// Writer 1
		go func() {
			for i := 0; i < 100; i++ {
				session := &Session{
					ID:          fmt.Sprintf("session-%d", i),
					StartedAt:   time.Now(),
					LastAttempt: time.Now(),
					Status:      StatusPending,
				}
				_ = store.Create(ctx, session)
			}
			done <- true
		}()

		// Writer 2 - Updates
		go func() {
			for i := 0; i < 50; i++ {
				session := &Session{
					ID:          fmt.Sprintf("session-%d", i),
					StartedAt:   time.Now(),
					LastAttempt: time.Now(),
					Status:      StatusCompleted,
				}
				_ = store.Update(ctx, session)
			}
			done <- true
		}()

		// Reader
		go func() {
			for i := 0; i < 100; i++ {
				_, _ = store.Get(ctx, fmt.Sprintf("session-%d", i%50))
				_, _ = store.GetActiveSessions(ctx)
			}
			done <- true
		}()

		// Wait for all goroutines
		for i := 0; i < 3; i++ {
			<-done
		}

		// Verify data integrity
		sessions, err := store.GetActiveSessions(ctx)
		require.NoError(t, err)
		assert.NotNil(t, sessions)
	})
}

func TestMemorySessionStore_ContextCancellation(t *testing.T) {
	store := NewMemorySessionStore()
	defer store.Stop()

	// Create canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	session := &Session{
		ID:          "test-session",
		StartedAt:   time.Now(),
		LastAttempt: time.Now(),
		Status:      StatusPending,
	}

	// Test all operations with canceled context
	err := store.Create(ctx, session)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")

	_, err = store.Get(ctx, "test-session")
	assert.Error(t, err)

	err = store.Update(ctx, session)
	assert.Error(t, err)

	_, err = store.GetActiveSessions(ctx)
	assert.Error(t, err)

	report := &Report{
		ID:             "test-report",
		SessionID:      "test-session",
		GeneratedAt:    time.Now(),
		PeriodStart:    time.Now().AddDate(0, -1, 0),
		PeriodEnd:      time.Now(),
		CoherenceScore: 0.8,
	}

	err = store.SaveReport(ctx, report)
	assert.Error(t, err)

	_, err = store.GetReport(ctx, "test-report")
	assert.Error(t, err)

	_, err = store.GetReportsByDateRange(ctx, time.Now().AddDate(0, -1, 0), time.Now())
	assert.Error(t, err)
}

// Helper function.
func stringPtr(s string) *string {
	return &s
}
