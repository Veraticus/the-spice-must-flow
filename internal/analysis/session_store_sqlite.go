package analysis

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// Ensure SQLiteSessionStore implements both SessionStore and ReportStore interfaces.
var _ SessionStore = (*SQLiteSessionStore)(nil)
var _ ReportStore = (*SQLiteSessionStore)(nil)

// SQLiteSessionStore provides SQLite-based persistence for analysis sessions and reports.
type SQLiteSessionStore struct {
	db *sql.DB
}

// NewSQLiteSessionStore creates a new SQLite-based session store.
func NewSQLiteSessionStore(db *sql.DB) *SQLiteSessionStore {
	return &SQLiteSessionStore{db: db}
}

// Create creates a new analysis session.
func (s *SQLiteSessionStore) Create(ctx context.Context, session *Session) error {
	if session.ID == "" {
		return fmt.Errorf("session ID is required")
	}

	query := `
		INSERT INTO analysis_sessions (
			id, started_at, last_attempt, completed_at, 
			status, attempts, error, report_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	var completedAt, errorStr, reportID sql.NullString
	if session.CompletedAt != nil {
		completedAt = sql.NullString{String: session.CompletedAt.Format(time.RFC3339), Valid: true}
	}
	if session.Error != nil {
		errorStr = sql.NullString{String: *session.Error, Valid: true}
	}
	if session.ReportID != nil {
		reportID = sql.NullString{String: *session.ReportID, Valid: true}
	}

	_, err := s.db.ExecContext(ctx, query,
		session.ID,
		session.StartedAt.Format(time.RFC3339),
		session.LastAttempt.Format(time.RFC3339),
		completedAt,
		string(session.Status),
		session.Attempts,
		errorStr,
		reportID,
	)

	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	slog.Debug("Created analysis session in database",
		"session_id", session.ID,
		"status", session.Status)

	return nil
}

// Get retrieves an analysis session by ID.
func (s *SQLiteSessionStore) Get(ctx context.Context, sessionID string) (*Session, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session ID is required")
	}

	query := `
		SELECT 
			started_at, last_attempt, completed_at,
			status, attempts, error, report_id
		FROM analysis_sessions
		WHERE id = ?
	`

	var (
		startedAtStr, lastAttemptStr       string
		completedAtStr, errorStr, reportID sql.NullString
		status                             string
		attempts                           int
	)

	row := s.db.QueryRowContext(ctx, query, sessionID)
	err := row.Scan(
		&startedAtStr,
		&lastAttemptStr,
		&completedAtStr,
		&status,
		&attempts,
		&errorStr,
		&reportID,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	session := &Session{
		ID:       sessionID,
		Status:   Status(status),
		Attempts: attempts,
	}

	// Parse timestamps
	session.StartedAt, err = time.Parse(time.RFC3339, startedAtStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse started_at: %w", err)
	}

	session.LastAttempt, err = time.Parse(time.RFC3339, lastAttemptStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse last_attempt: %w", err)
	}

	if completedAtStr.Valid {
		t, err := time.Parse(time.RFC3339, completedAtStr.String)
		if err != nil {
			return nil, fmt.Errorf("failed to parse completed_at: %w", err)
		}
		session.CompletedAt = &t
	}

	if errorStr.Valid {
		session.Error = &errorStr.String
	}

	if reportID.Valid {
		session.ReportID = &reportID.String
	}

	slog.Debug("Retrieved analysis session from database",
		"session_id", sessionID,
		"status", session.Status,
		"attempts", session.Attempts)

	return session, nil
}

// Update updates an existing analysis session.
func (s *SQLiteSessionStore) Update(ctx context.Context, session *Session) error {
	if session.ID == "" {
		return fmt.Errorf("session ID is required")
	}

	query := `
		UPDATE analysis_sessions SET
			last_attempt = ?,
			completed_at = ?,
			status = ?,
			attempts = ?,
			error = ?,
			report_id = ?
		WHERE id = ?
	`

	var completedAt, errorStr, reportID sql.NullString
	if session.CompletedAt != nil {
		completedAt = sql.NullString{String: session.CompletedAt.Format(time.RFC3339), Valid: true}
	}
	if session.Error != nil {
		errorStr = sql.NullString{String: *session.Error, Valid: true}
	}
	if session.ReportID != nil {
		reportID = sql.NullString{String: *session.ReportID, Valid: true}
	}

	result, err := s.db.ExecContext(ctx, query,
		session.LastAttempt.Format(time.RFC3339),
		completedAt,
		string(session.Status),
		session.Attempts,
		errorStr,
		reportID,
		session.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update session: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("session not found: %s", session.ID)
	}

	slog.Debug("Updated analysis session in database",
		"session_id", session.ID,
		"status", session.Status,
		"attempts", session.Attempts)

	return nil
}

// SaveReport stores an analysis report.
func (s *SQLiteSessionStore) SaveReport(ctx context.Context, report *Report) error {
	if report.ID == "" {
		return fmt.Errorf("report ID is required")
	}
	if report.SessionID == "" {
		return fmt.Errorf("session ID is required")
	}

	// Use a transaction to ensure all related data is saved atomically
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback() // Will be no-op if already committed
	}()

	// Save the main report
	query := `
		INSERT INTO analysis_reports (
			id, session_id, generated_at, period_start, period_end,
			coherence_score, insights
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	insightsJSON, err := json.Marshal(report.Insights)
	if err != nil {
		return fmt.Errorf("failed to marshal insights: %w", err)
	}

	_, err = tx.ExecContext(ctx, query,
		report.ID,
		report.SessionID,
		report.GeneratedAt.Format(time.RFC3339),
		report.PeriodStart.Format(time.RFC3339),
		report.PeriodEnd.Format(time.RFC3339),
		report.CoherenceScore,
		string(insightsJSON),
	)
	if err != nil {
		return fmt.Errorf("failed to save report: %w", err)
	}

	// Save issues and their fixes
	for _, issue := range report.Issues {
		if err := s.saveIssue(ctx, tx, report.ID, issue); err != nil {
			return fmt.Errorf("failed to save issue: %w", err)
		}
	}

	// Save suggested patterns
	for _, pattern := range report.SuggestedPatterns {
		if err := s.saveSuggestedPattern(ctx, tx, report.ID, pattern); err != nil {
			return fmt.Errorf("failed to save suggested pattern: %w", err)
		}
	}

	// Save category stats
	for categoryName, stat := range report.CategorySummary {
		if err := s.saveCategoryStat(ctx, tx, report.ID, categoryName, stat); err != nil {
			return fmt.Errorf("failed to save category stat: %w", err)
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	slog.Info("Saved analysis report to database",
		"report_id", report.ID,
		"session_id", report.SessionID,
		"issues", len(report.Issues),
		"patterns", len(report.SuggestedPatterns))

	return nil
}

// GetReport retrieves a report by ID.
func (s *SQLiteSessionStore) GetReport(ctx context.Context, reportID string) (*Report, error) {
	if reportID == "" {
		return nil, fmt.Errorf("report ID is required")
	}

	// Get the main report
	query := `
		SELECT 
			session_id, generated_at, period_start, period_end,
			coherence_score, insights
		FROM analysis_reports
		WHERE id = ?
	`

	var (
		generatedAtStr, periodStartStr, periodEndStr string
		insightsJSON                                 string
	)

	report := &Report{
		ID: reportID,
	}

	row := s.db.QueryRowContext(ctx, query, reportID)
	err := row.Scan(
		&report.SessionID,
		&generatedAtStr,
		&periodStartStr,
		&periodEndStr,
		&report.CoherenceScore,
		&insightsJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("report not found: %s", reportID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get report: %w", err)
	}

	// Parse timestamps
	report.GeneratedAt, err = time.Parse(time.RFC3339, generatedAtStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse generated_at: %w", err)
	}

	report.PeriodStart, err = time.Parse(time.RFC3339, periodStartStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse period_start: %w", err)
	}

	report.PeriodEnd, err = time.Parse(time.RFC3339, periodEndStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse period_end: %w", err)
	}

	// Parse insights
	if unmarshalErr := json.Unmarshal([]byte(insightsJSON), &report.Insights); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to unmarshal insights: %w", unmarshalErr)
	}

	// Load related data
	var loadErr error
	report.Issues, loadErr = s.loadIssues(ctx, reportID)
	if loadErr != nil {
		return nil, fmt.Errorf("failed to load issues: %w", loadErr)
	}

	report.SuggestedPatterns, loadErr = s.loadSuggestedPatterns(ctx, reportID)
	if loadErr != nil {
		return nil, fmt.Errorf("failed to load suggested patterns: %w", loadErr)
	}

	report.CategorySummary, loadErr = s.loadCategoryStats(ctx, reportID)
	if loadErr != nil {
		return nil, fmt.Errorf("failed to load category stats: %w", loadErr)
	}

	// Report is now fully loaded

	slog.Debug("Retrieved analysis report from database",
		"report_id", reportID,
		"issues", len(report.Issues),
		"patterns", len(report.SuggestedPatterns))

	return report, nil
}

// Helper methods for saving related data

func (s *SQLiteSessionStore) saveIssue(ctx context.Context, tx *sql.Tx, reportID string, issue Issue) error {
	// Debug log to see what issue type is being saved
	slog.Debug("Saving issue to database",
		"issue_id", issue.ID,
		"issue_type", string(issue.Type),
		"severity", string(issue.Severity),
		"description", issue.Description)

	query := `
		INSERT INTO analysis_issues (
			id, report_id, type, severity, description,
			current_category, suggested_category, transaction_ids,
			affected_count, confidence
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	txnIDsJSON, err := json.Marshal(issue.TransactionIDs)
	if err != nil {
		return fmt.Errorf("failed to marshal transaction IDs: %w", err)
	}

	var currentCat, suggestedCat sql.NullString
	if issue.CurrentCategory != nil {
		currentCat = sql.NullString{String: *issue.CurrentCategory, Valid: true}
	}
	if issue.SuggestedCategory != nil {
		suggestedCat = sql.NullString{String: *issue.SuggestedCategory, Valid: true}
	}

	_, err = tx.ExecContext(ctx, query,
		issue.ID,
		reportID,
		string(issue.Type),
		string(issue.Severity),
		issue.Description,
		currentCat,
		suggestedCat,
		string(txnIDsJSON),
		issue.AffectedCount,
		issue.Confidence,
	)

	if err != nil {
		return err
	}

	// Save fix if present
	if issue.Fix != nil {
		return s.saveFix(ctx, tx, issue.ID, *issue.Fix)
	}

	return nil
}

func (s *SQLiteSessionStore) saveFix(ctx context.Context, tx *sql.Tx, issueID string, fix Fix) error {
	query := `
		INSERT INTO analysis_fixes (
			id, issue_id, type, description, data, applied, applied_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	dataJSON, err := json.Marshal(fix.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal fix data: %w", err)
	}

	var appliedAt sql.NullString
	if fix.AppliedAt != nil {
		appliedAt = sql.NullString{String: fix.AppliedAt.Format(time.RFC3339), Valid: true}
	}

	_, err = tx.ExecContext(ctx, query,
		fix.ID,
		issueID,
		fix.Type,
		fix.Description,
		string(dataJSON),
		fix.AppliedAt != nil,
		appliedAt,
	)

	return err
}

func (s *SQLiteSessionStore) saveSuggestedPattern(ctx context.Context, tx *sql.Tx, reportID string, pattern SuggestedPattern) error {
	query := `
		INSERT INTO analysis_suggested_patterns (
			id, report_id, name, description, impact,
			pattern, example_txn_ids, match_count, confidence
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	patternJSON, err := json.Marshal(pattern.Pattern)
	if err != nil {
		return fmt.Errorf("failed to marshal pattern: %w", err)
	}

	exampleIDsJSON, err := json.Marshal(pattern.ExampleTxnIDs)
	if err != nil {
		return fmt.Errorf("failed to marshal example IDs: %w", err)
	}

	_, err = tx.ExecContext(ctx, query,
		pattern.ID,
		reportID,
		pattern.Name,
		pattern.Description,
		pattern.Impact,
		string(patternJSON),
		string(exampleIDsJSON),
		pattern.MatchCount,
		pattern.Confidence,
	)

	return err
}

func (s *SQLiteSessionStore) saveCategoryStat(ctx context.Context, tx *sql.Tx, reportID string, categoryName string, stat CategoryStat) error {
	query := `
		INSERT INTO analysis_category_stats (
			report_id, category_id, category_name,
			transaction_count, total_amount, consistency, issues
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err := tx.ExecContext(ctx, query,
		reportID,
		stat.CategoryID,
		categoryName,
		stat.TransactionCount,
		stat.TotalAmount,
		stat.Consistency,
		stat.Issues,
	)

	return err
}

// Helper methods for loading related data

func (s *SQLiteSessionStore) loadIssues(ctx context.Context, reportID string) ([]Issue, error) {
	query := `
		SELECT 
			i.id, i.type, i.severity, i.description,
			i.current_category, i.suggested_category, i.transaction_ids,
			i.affected_count, i.confidence,
			f.id, f.type, f.description, f.data, f.applied_at
		FROM analysis_issues i
		LEFT JOIN analysis_fixes f ON f.issue_id = i.id
		WHERE i.report_id = ?
		ORDER BY 
			CASE i.severity 
				WHEN 'critical' THEN 1 
				WHEN 'high' THEN 2 
				WHEN 'medium' THEN 3 
				WHEN 'low' THEN 4 
			END,
			i.affected_count DESC
	`

	rows, err := s.db.QueryContext(ctx, query, reportID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var issues []Issue
	for rows.Next() {
		var (
			issue                                Issue
			currentCat, suggestedCat             sql.NullString
			txnIDsJSON                           string
			fixID, fixType, fixDesc, fixDataJSON sql.NullString
			fixAppliedAt                         sql.NullString
		)

		err := rows.Scan(
			&issue.ID,
			&issue.Type,
			&issue.Severity,
			&issue.Description,
			&currentCat,
			&suggestedCat,
			&txnIDsJSON,
			&issue.AffectedCount,
			&issue.Confidence,
			&fixID,
			&fixType,
			&fixDesc,
			&fixDataJSON,
			&fixAppliedAt,
		)
		if err != nil {
			return nil, err
		}

		// Parse optional fields
		if currentCat.Valid {
			issue.CurrentCategory = &currentCat.String
		}
		if suggestedCat.Valid {
			issue.SuggestedCategory = &suggestedCat.String
		}

		// Parse transaction IDs
		if err := json.Unmarshal([]byte(txnIDsJSON), &issue.TransactionIDs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal transaction IDs: %w", err)
		}

		// Parse fix if present
		if fixID.Valid {
			fix := &Fix{
				ID:          fixID.String,
				IssueID:     issue.ID,
				Type:        fixType.String,
				Description: fixDesc.String,
			}

			if err := json.Unmarshal([]byte(fixDataJSON.String), &fix.Data); err != nil {
				return nil, fmt.Errorf("failed to unmarshal fix data: %w", err)
			}

			if fixAppliedAt.Valid {
				t, err := time.Parse(time.RFC3339, fixAppliedAt.String)
				if err != nil {
					return nil, fmt.Errorf("failed to parse fix applied_at: %w", err)
				}
				fix.AppliedAt = &t
			}

			issue.Fix = fix
		}

		issues = append(issues, issue)
	}

	return issues, rows.Err()
}

func (s *SQLiteSessionStore) loadSuggestedPatterns(ctx context.Context, reportID string) ([]SuggestedPattern, error) {
	query := `
		SELECT 
			id, name, description, impact,
			pattern, example_txn_ids, match_count, confidence
		FROM analysis_suggested_patterns
		WHERE report_id = ?
		ORDER BY confidence DESC, match_count DESC
	`

	rows, err := s.db.QueryContext(ctx, query, reportID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var patterns []SuggestedPattern
	for rows.Next() {
		var (
			pattern                     SuggestedPattern
			patternJSON, exampleIDsJSON string
		)

		err := rows.Scan(
			&pattern.ID,
			&pattern.Name,
			&pattern.Description,
			&pattern.Impact,
			&patternJSON,
			&exampleIDsJSON,
			&pattern.MatchCount,
			&pattern.Confidence,
		)
		if err != nil {
			return nil, err
		}

		// Parse pattern rule
		if err := json.Unmarshal([]byte(patternJSON), &pattern.Pattern); err != nil {
			return nil, fmt.Errorf("failed to unmarshal pattern: %w", err)
		}

		// Parse example IDs
		if err := json.Unmarshal([]byte(exampleIDsJSON), &pattern.ExampleTxnIDs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal example IDs: %w", err)
		}

		patterns = append(patterns, pattern)
	}

	return patterns, rows.Err()
}

func (s *SQLiteSessionStore) loadCategoryStats(ctx context.Context, reportID string) (map[string]CategoryStat, error) {
	query := `
		SELECT 
			category_id, category_name,
			transaction_count, total_amount, consistency, issues
		FROM analysis_category_stats
		WHERE report_id = ?
	`

	rows, err := s.db.QueryContext(ctx, query, reportID)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	stats := make(map[string]CategoryStat)
	for rows.Next() {
		var (
			categoryName string
			stat         CategoryStat
		)

		err := rows.Scan(
			&stat.CategoryID,
			&categoryName,
			&stat.TransactionCount,
			&stat.TotalAmount,
			&stat.Consistency,
			&stat.Issues,
		)
		if err != nil {
			return nil, err
		}

		stat.CategoryName = categoryName
		stats[categoryName] = stat
	}

	return stats, rows.Err()
}
