package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/joshsymonds/the-spice-must-flow/internal/engine"
	"github.com/joshsymonds/the-spice-must-flow/internal/model"
	"github.com/joshsymonds/the-spice-must-flow/internal/service"
	"github.com/schollz/progressbar/v3"
)

// Prompter implements the interactive CLI prompting interface for transaction classification.
type Prompter struct {
	startTime         time.Time
	writer            io.Writer
	reader            *bufio.Reader
	progressBar       *progressbar.ProgressBar
	categoryHistory   map[string][]string
	recentCategories  []string
	stats             service.CompletionStats
	totalTransactions int
	processedCount    int
	statsMutex        sync.RWMutex
	historyMutex      sync.RWMutex
}

// NewCLIPrompter creates a new CLI prompter with the given reader and writer.
func NewCLIPrompter(reader io.Reader, writer io.Writer) *Prompter {
	if reader == nil {
		reader = os.Stdin
	}
	if writer == nil {
		writer = os.Stdout
	}

	return &Prompter{
		reader:          bufio.NewReader(reader),
		writer:          writer,
		categoryHistory: make(map[string][]string),
		startTime:       time.Now(),
	}
}

// ConfirmClassification prompts the user to confirm or modify a single transaction classification.
func (p *Prompter) ConfirmClassification(ctx context.Context, pending model.PendingClassification) (model.Classification, error) {
	select {
	case <-ctx.Done():
		return model.Classification{}, ctx.Err()
	default:
	}

	p.updateProgress()

	content := p.formatSingleTransaction(pending)
	if _, err := fmt.Fprintln(p.writer, RenderBox("Transaction Details", content)); err != nil {
		return model.Classification{}, fmt.Errorf("failed to write transaction box: %w", err)
	}

	if _, err := fmt.Fprintln(p.writer, FormatPrompt("Category options:")); err != nil {
		return model.Classification{}, fmt.Errorf("failed to write category options: %w", err)
	}
	
	if pending.IsNewCategory {
		if _, err := fmt.Fprintf(p.writer, "  [A] Create and use new category: %s\n", WarningStyle.Render(pending.SuggestedCategory)); err != nil {
			return model.Classification{}, fmt.Errorf("failed to write new category option: %w", err)
		}
		if _, err := fmt.Fprintln(p.writer, "  [E] Use existing category instead"); err != nil {
			return model.Classification{}, fmt.Errorf("failed to write existing option: %w", err)
		}
	} else {
		if _, err := fmt.Fprintf(p.writer, "  [A] Accept AI suggestion: %s\n", SuccessStyle.Render(pending.SuggestedCategory)); err != nil {
			return model.Classification{}, fmt.Errorf("failed to write AI suggestion: %w", err)
		}
	}
	
	if _, err := fmt.Fprintln(p.writer, "  [C] Enter custom category"); err != nil {
		return model.Classification{}, fmt.Errorf("failed to write custom option: %w", err)
	}
	if _, err := fmt.Fprintln(p.writer, "  [S] Skip this transaction"); err != nil {
		return model.Classification{}, fmt.Errorf("failed to write skip option: %w", err)
	}
	if _, err := fmt.Fprintln(p.writer); err != nil {
		return model.Classification{}, fmt.Errorf("failed to write newline: %w", err)
	}

	var validChoices []string
	if pending.IsNewCategory {
		validChoices = []string{"a", "e", "c", "s"}
	} else {
		validChoices = []string{"a", "c", "s"}
	}
	
	choice, err := p.promptChoice(ctx, "Choice", validChoices)
	if err != nil {
		return model.Classification{}, err
	}

	classification := model.Classification{
		Transaction:  pending.Transaction,
		Confidence:   pending.Confidence,
		ClassifiedAt: time.Now(),
	}

	switch choice {
	case "a":
		classification.Category = pending.SuggestedCategory
		classification.Status = model.StatusClassifiedByAI
		p.trackCategorization(pending.Transaction.MerchantName, pending.SuggestedCategory)
		p.incrementStats(false, false)
		if pending.IsNewCategory {
			if _, err := fmt.Fprintf(p.writer, FormatSuccess("✓ Will create new category: %s\n"), pending.SuggestedCategory); err != nil {
				slog.Warn("Failed to write new category confirmation", "error", err)
			}
		}
	case "e":
		// This option is only available for new category suggestions
		if pending.IsNewCategory {
			category, err := p.promptCustomCategory(ctx)
			if err != nil {
				return model.Classification{}, err
			}
			classification.Category = category
			classification.Status = model.StatusUserModified
			classification.Confidence = 1.0
			p.trackCategorization(pending.Transaction.MerchantName, category)
			p.incrementStats(true, false)
		}
	case "c":
		category, err := p.promptCustomCategory(ctx)
		if err != nil {
			return model.Classification{}, err
		}
		classification.Category = category
		classification.Status = model.StatusUserModified
		p.trackCategorization(pending.Transaction.MerchantName, category)
		p.incrementStats(true, false)
	case "s":
		classification.Status = model.StatusUnclassified
	}

	return classification, nil
}

// BatchConfirmClassifications prompts the user to confirm or modify multiple transaction classifications.
func (p *Prompter) BatchConfirmClassifications(ctx context.Context, pending []model.PendingClassification) ([]model.Classification, error) {
	if len(pending) == 0 {
		return []model.Classification{}, nil
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	p.updateProgress()

	merchantName := pending[0].Transaction.MerchantName
	pattern := p.detectPattern(merchantName)

	content := p.formatBatchSummary(pending, pattern)
	if _, err := fmt.Fprintln(p.writer, RenderBox("Batch Review", content)); err != nil {
		return nil, fmt.Errorf("failed to write batch review box: %w", err)
	}

	if _, err := fmt.Fprintln(p.writer, FormatPrompt("Options:")); err != nil {
		return nil, fmt.Errorf("failed to write options prompt: %w", err)
	}
	
	if pending[0].IsNewCategory {
		if _, err := fmt.Fprintf(p.writer, "  [A] Create and use new category '%s' for all %d transactions\n", 
			pending[0].SuggestedCategory, len(pending)); err != nil {
			return nil, fmt.Errorf("failed to write new category accept option: %w", err)
		}
		if _, err := fmt.Fprintln(p.writer, "  [E] Use existing category for all"); err != nil {
			return nil, fmt.Errorf("failed to write existing category option: %w", err)
		}
	} else {
		if _, err := fmt.Fprintf(p.writer, "  [A] Accept for all %d transactions\n", len(pending)); err != nil {
			return nil, fmt.Errorf("failed to write batch accept option: %w", err)
		}
	}
	
	if _, err := fmt.Fprintln(p.writer, "  [C] Set custom category for all"); err != nil {
		return nil, fmt.Errorf("failed to write custom category option: %w", err)
	}
	if _, err := fmt.Fprintln(p.writer, "  [R] Review each transaction individually"); err != nil {
		return nil, fmt.Errorf("failed to write review option: %w", err)
	}
	if _, err := fmt.Fprintln(p.writer, "  [S] Skip all transactions"); err != nil {
		return nil, fmt.Errorf("failed to write skip all option: %w", err)
	}
	if _, err := fmt.Fprintln(p.writer); err != nil {
		return nil, fmt.Errorf("failed to write newline: %w", err)
	}

	var validChoices []string
	var promptText string
	if pending[0].IsNewCategory {
		validChoices = []string{"a", "e", "c", "r", "s"}
		promptText = "Choice [A/E/C/R/S]"
	} else {
		validChoices = []string{"a", "c", "r", "s"}
		promptText = "Choice [A/C/R/S]"
	}
	
	choice, err := p.promptChoice(ctx, promptText, validChoices)
	if err != nil {
		return nil, err
	}

	switch choice {
	case "a":
		if pending[0].IsNewCategory {
			if _, err := fmt.Fprintf(p.writer, FormatSuccess("✓ Will create new category: %s\n"), pending[0].SuggestedCategory); err != nil {
				slog.Warn("Failed to write new category confirmation", "error", err)
			}
		}
		return p.acceptAllClassifications(pending)
	case "e":
		// This option is only available for new category suggestions
		if pending[0].IsNewCategory {
			return p.customCategoryForAll(ctx, pending)
		}
	case "c":
		return p.customCategoryForAll(ctx, pending)
	case "r":
		return p.reviewEachTransaction(ctx, pending)
	case "s":
		return p.skipAllClassifications(pending)
	}

	return nil, fmt.Errorf("unexpected choice: %s", choice)
}

// GetCompletionStats returns statistics about the classification session.
func (p *Prompter) GetCompletionStats() service.CompletionStats {
	p.statsMutex.RLock()
	defer p.statsMutex.RUnlock()

	stats := p.stats
	stats.Duration = time.Since(p.startTime)
	return stats
}

// SetTotalTransactions sets the total number of transactions to be processed.
func (p *Prompter) SetTotalTransactions(total int) {
	p.totalTransactions = total
	p.initProgressBar()
}

// ShowCompletion displays the completion summary to the user.
func (p *Prompter) ShowCompletion() {
	if p.progressBar != nil {
		if err := p.progressBar.Finish(); err != nil {
			slog.Warn("Failed to finish progress bar", "error", err)
		}
		if _, err := fmt.Fprintln(p.writer); err != nil {
			slog.Warn("Failed to write newline", "error", err)
		}
	}

	stats := p.GetCompletionStats()
	timeSaved := p.calculateTimeSaved(stats)

	summary := fmt.Sprintf("%s Classification Complete!\n\n", SpiceIcon) +
		"📊 Statistics:\n" +
		fmt.Sprintf("  • Total transactions: %d\n", stats.TotalTransactions) +
		fmt.Sprintf("  • Auto-classified: %d (%.1f%%)\n", stats.AutoClassified,
			float64(stats.AutoClassified)/float64(stats.TotalTransactions)*100) +
		fmt.Sprintf("  • User-classified: %d\n", stats.UserClassified) +
		fmt.Sprintf("  • New vendor rules: %d\n", stats.NewVendorRules) +
		fmt.Sprintf("  • Time taken: %s\n", stats.Duration.Round(time.Second)) +
		fmt.Sprintf("  • Time saved: ~%s %s\n", timeSaved, RobotIcon)

	if _, err := fmt.Fprintln(p.writer, RenderBox("Classification Complete", summary)); err != nil {
		slog.Warn("Failed to write completion box", "error", err)
	}
}

func (p *Prompter) initProgressBar() {
	p.progressBar = progressbar.NewOptions(p.totalTransactions,
		progressbar.OptionSetWriter(p.writer),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionShowElapsedTimeOnFinish(),
		progressbar.OptionSetWidth(40),
		progressbar.OptionSetDescription("[cyan][bold]Classifying transactions...[reset]"),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionOnCompletion(func() {
			if _, err := fmt.Fprintln(p.writer); err != nil {
				slog.Warn("Failed to write newline after progress bar", "error", err)
			}
		}),
	)
}

func (p *Prompter) updateProgress() {
	p.processedCount++
	if p.progressBar != nil {
		if err := p.progressBar.Add(1); err != nil {
			slog.Warn("Failed to update progress bar", "error", err)
		}
	}
}

func (p *Prompter) formatSingleTransaction(pending model.PendingClassification) string {
	t := pending.Transaction

	header := TitleStyle.Render(fmt.Sprintf("Transaction Review: %s", t.MerchantName))

	details := fmt.Sprintf("%s Details:\n", InfoIcon) +
		fmt.Sprintf("  Date: %s\n", t.Date.Format("Jan 2, 2006")) +
		fmt.Sprintf("  Amount: $%.2f\n", t.Amount) +
		fmt.Sprintf("  Description: %s\n", t.Name)

	var suggestion string
	if pending.IsNewCategory {
		suggestion = fmt.Sprintf("\n%s AI suggests NEW category: %s (%.0f%% confidence)",
			RobotIcon,
			WarningStyle.Render(pending.SuggestedCategory),
			pending.Confidence*100)
		suggestion += fmt.Sprintf("\n  %s This is a new category suggestion", InfoIcon)
	} else {
		suggestion = fmt.Sprintf("\n%s AI Suggestion: %s (%.0f%% confidence)",
			RobotIcon,
			SuccessStyle.Render(pending.SuggestedCategory),
			pending.Confidence*100)
	}

	if pending.SimilarCount > 0 {
		suggestion += fmt.Sprintf("\n  %s Similar transactions: %d", InfoIcon, pending.SimilarCount)
	}

	return header + "\n\n" + details + suggestion
}

func (p *Prompter) formatBatchSummary(pending []model.PendingClassification, pattern string) string {
	merchantName := pending[0].Transaction.MerchantName
	suggestedCategory := pending[0].SuggestedCategory

	var totalAmount float64
	var minDate, maxDate time.Time

	for i, pc := range pending {
		totalAmount += pc.Transaction.Amount
		if i == 0 || pc.Transaction.Date.Before(minDate) {
			minDate = pc.Transaction.Date
		}
		if i == 0 || pc.Transaction.Date.After(maxDate) {
			maxDate = pc.Transaction.Date
		}
	}

	header := TitleStyle.Render(fmt.Sprintf("Batch Review: %s", merchantName))

	summary := fmt.Sprintf("\n%s Summary:\n", InfoIcon) +
		fmt.Sprintf("  Transactions: %d\n", len(pending)) +
		fmt.Sprintf("  Total: $%.2f\n", totalAmount) +
		fmt.Sprintf("  Date range: %s to %s\n",
			minDate.Format("Jan 2"),
			maxDate.Format("Jan 2, 2006"))

	var suggestion string
	if pending[0].IsNewCategory {
		suggestion = fmt.Sprintf("\n%s AI suggests NEW category: %s",
			RobotIcon,
			WarningStyle.Render(suggestedCategory))
		suggestion += fmt.Sprintf("\n%s This is a new category suggestion", InfoIcon)
	} else {
		suggestion = fmt.Sprintf("\n%s AI suggests: %s",
			RobotIcon,
			SuccessStyle.Render(suggestedCategory))
	}

	if pattern != "" {
		suggestion += fmt.Sprintf("\n%s Pattern detected: %s", CheckIcon, pattern)
	}

	samples := p.formatTransactionSamples(pending)

	return header + summary + suggestion + samples
}

func (p *Prompter) formatTransactionSamples(pending []model.PendingClassification) string {
	if len(pending) <= 3 {
		return ""
	}

	samples := fmt.Sprintf("\n\n%s Sample transactions:\n", InfoIcon)

	for i := 0; i < 3 && i < len(pending); i++ {
		t := pending[i].Transaction
		samples += fmt.Sprintf("  • %s - $%.2f\n",
			t.Date.Format("Jan 2"),
			t.Amount)
	}

	if len(pending) > 3 {
		samples += fmt.Sprintf("  • ... and %d more\n", len(pending)-3)
	}

	return samples
}

func (p *Prompter) detectPattern(merchantName string) string {
	p.historyMutex.RLock()
	defer p.historyMutex.RUnlock()

	history, exists := p.categoryHistory[merchantName]
	if !exists || len(history) < 3 {
		return ""
	}

	lastCategory := history[len(history)-1]
	count := 0
	for i := len(history) - 1; i >= 0 && history[i] == lastCategory; i-- {
		count++
	}

	if count >= 3 {
		return fmt.Sprintf("Last %d were categorized as %s", count, lastCategory)
	}

	return ""
}

func (p *Prompter) trackCategorization(merchantName, category string) {
	p.historyMutex.Lock()
	defer p.historyMutex.Unlock()

	p.categoryHistory[merchantName] = append(p.categoryHistory[merchantName], category)

	p.recentCategories = append([]string{category}, p.recentCategories...)
	if len(p.recentCategories) > 10 {
		p.recentCategories = p.recentCategories[:10]
	}
}

func (p *Prompter) promptChoice(ctx context.Context, prompt string, validChoices []string) (string, error) {
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		if _, err := fmt.Fprintf(p.writer, "%s: ", FormatPrompt(prompt)); err != nil {
			return "", fmt.Errorf("failed to write prompt: %w", err)
		}

		input, err := p.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return "", fmt.Errorf("input terminated")
			}
			return "", err
		}

		choice := strings.ToLower(strings.TrimSpace(input))

		for _, valid := range validChoices {
			if choice == valid {
				return choice, nil
			}
		}

		if _, err := fmt.Fprintln(p.writer, FormatError("Invalid choice. Please try again.")); err != nil {
			slog.Warn("Failed to write error message", "error", err)
		}
	}
}

func (p *Prompter) promptCustomCategory(ctx context.Context) (string, error) {
	if _, err := fmt.Fprintln(p.writer); err != nil {
		return "", fmt.Errorf("failed to write newline: %w", err)
	}

	if len(p.recentCategories) > 0 {
		if _, err := fmt.Fprintln(p.writer, FormatInfo("Recent categories:")); err != nil {
			return "", fmt.Errorf("failed to write recent categories header: %w", err)
		}
		seen := make(map[string]bool)
		for _, cat := range p.recentCategories {
			if !seen[cat] {
				if _, err := fmt.Fprintf(p.writer, "  • %s\n", cat); err != nil {
					slog.Warn("Failed to write recent category", "error", err)
				}
				seen[cat] = true
			}
		}
		if _, err := fmt.Fprintln(p.writer); err != nil {
			return "", fmt.Errorf("failed to write newline after categories: %w", err)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		if _, err := fmt.Fprint(p.writer, FormatPrompt("Enter category: ")); err != nil {
			return "", fmt.Errorf("failed to write category prompt: %w", err)
		}

		input, err := p.reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		category := strings.TrimSpace(input)
		if category == "" {
			if _, err := fmt.Fprintln(p.writer, FormatError("Category cannot be empty. Please try again.")); err != nil {
				slog.Warn("Failed to write empty category error", "error", err)
			}
			continue
		}

		return category, nil
	}
}

func (p *Prompter) acceptAllClassifications(pending []model.PendingClassification) ([]model.Classification, error) {
	classifications := make([]model.Classification, len(pending))

	for i, pc := range pending {
		classifications[i] = model.Classification{
			Transaction:  pc.Transaction,
			Category:     pc.SuggestedCategory,
			Status:       model.StatusClassifiedByAI,
			Confidence:   pc.Confidence,
			ClassifiedAt: time.Now(),
		}
		p.trackCategorization(pc.Transaction.MerchantName, pc.SuggestedCategory)
	}

	p.incrementBatchStats(len(pending), false, true)
	if _, err := fmt.Fprintln(p.writer, FormatSuccess(fmt.Sprintf("✓ Classified %d transactions as %s",
		len(pending), pending[0].SuggestedCategory))); err != nil {
		slog.Warn("Failed to write success message", "error", err)
	}

	return classifications, nil
}

func (p *Prompter) customCategoryForAll(ctx context.Context, pending []model.PendingClassification) ([]model.Classification, error) {
	category, err := p.promptCustomCategory(ctx)
	if err != nil {
		return nil, err
	}

	classifications := make([]model.Classification, len(pending))

	for i, pc := range pending {
		classifications[i] = model.Classification{
			Transaction:  pc.Transaction,
			Category:     category,
			Status:       model.StatusUserModified,
			Confidence:   1.0,
			ClassifiedAt: time.Now(),
		}
		p.trackCategorization(pc.Transaction.MerchantName, category)
	}

	p.incrementBatchStats(len(pending), true, true)
	if _, err := fmt.Fprintln(p.writer, FormatSuccess(fmt.Sprintf("✓ Classified %d transactions as %s",
		len(pending), category))); err != nil {
		slog.Warn("Failed to write custom category success", "error", err)
	}

	return classifications, nil
}

func (p *Prompter) reviewEachTransaction(ctx context.Context, pending []model.PendingClassification) ([]model.Classification, error) {
	if _, err := fmt.Fprintln(p.writer, FormatInfo(fmt.Sprintf("Reviewing %d transactions individually...", len(pending)))); err != nil {
		return nil, fmt.Errorf("failed to write review info: %w", err)
	}

	classifications := make([]model.Classification, 0, len(pending))

	for i, pc := range pending {
		if _, err := fmt.Fprintf(p.writer, "\n[%d/%d] ", i+1, len(pending)); err != nil {
			slog.Warn("Failed to write progress", "error", err)
		}

		classification, err := p.ConfirmClassification(ctx, pc)
		if err != nil {
			return nil, err
		}

		classifications = append(classifications, classification)
	}

	return classifications, nil
}

func (p *Prompter) skipAllClassifications(pending []model.PendingClassification) ([]model.Classification, error) {
	classifications := make([]model.Classification, len(pending))

	for i, pc := range pending {
		classifications[i] = model.Classification{
			Transaction:  pc.Transaction,
			Status:       model.StatusUnclassified,
			ClassifiedAt: time.Now(),
		}
	}

	if _, err := fmt.Fprintln(p.writer, FormatWarning(fmt.Sprintf("⚠ Skipped %d transactions", len(pending)))); err != nil {
		slog.Warn("Failed to write skip warning", "error", err)
	}

	return classifications, nil
}

func (p *Prompter) incrementStats(userModified bool, isVendorRule bool) {
	p.statsMutex.Lock()
	defer p.statsMutex.Unlock()

	p.stats.TotalTransactions++

	if userModified {
		p.stats.UserClassified++
	} else {
		p.stats.AutoClassified++
	}

	if isVendorRule {
		p.stats.NewVendorRules++
	}
}

func (p *Prompter) incrementBatchStats(count int, userModified bool, isVendorRule bool) {
	p.statsMutex.Lock()
	defer p.statsMutex.Unlock()

	p.stats.TotalTransactions += count

	if userModified {
		p.stats.UserClassified += count
	} else {
		p.stats.AutoClassified += count
	}

	if isVendorRule {
		p.stats.NewVendorRules++
	}
}

func (p *Prompter) calculateTimeSaved(stats service.CompletionStats) string {
	avgSecondsPerTransaction := 5.0

	timeSavedSeconds := float64(stats.AutoClassified) * avgSecondsPerTransaction

	switch {
	case timeSavedSeconds < 60:
		return fmt.Sprintf("%.0f seconds", timeSavedSeconds)
	case timeSavedSeconds < 3600:
		return fmt.Sprintf("%.1f minutes", timeSavedSeconds/60)
	default:
		return fmt.Sprintf("%.1f hours", timeSavedSeconds/3600)
	}
}

// Ensure Prompter implements the engine.Prompter interface.
var _ engine.Prompter = (*Prompter)(nil)
