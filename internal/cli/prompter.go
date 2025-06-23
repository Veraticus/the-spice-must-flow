package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Veraticus/the-spice-must-flow/internal/engine"
	"github.com/Veraticus/the-spice-must-flow/internal/model"
	"github.com/Veraticus/the-spice-must-flow/internal/service"
	"github.com/schollz/progressbar/v3"
)

// Prompter implements the interactive CLI prompting interface for transaction classification.
type Prompter struct {
	startTime         time.Time
	writer            io.Writer
	ctx               context.Context
	reader            *NonBlockingReader
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
		reader:          NewNonBlockingReader(reader),
		writer:          writer,
		categoryHistory: make(map[string][]string),
		startTime:       time.Now(),
	}
}

// Start initializes the non-blocking reader with the given context.
func (p *Prompter) Start(ctx context.Context) {
	p.ctx = ctx
	p.reader.Start(ctx)
}

// Close cleans up the prompter resources.
func (p *Prompter) Close() {
	if p.reader != nil {
		p.reader.Close()
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

	// Add spacing after progress bar
	if _, err := fmt.Fprintln(p.writer); err != nil {
		slog.Warn("Failed to write newline", "error", err)
	}

	content := p.formatSingleTransaction(pending)
	if _, err := fmt.Fprintln(p.writer, RenderBox("Transaction Details", content)); err != nil {
		return model.Classification{}, fmt.Errorf("failed to write transaction box: %w", err)
	}

	// First handle direction confirmation
	direction := pending.SuggestedDirection
	if pending.DirectionConfidence < 0.8 || pending.SuggestedDirection == "" {
		// Low confidence or missing direction - prompt user
		if _, err := fmt.Fprintln(p.writer, FormatPrompt("Transaction Direction:")); err != nil {
			return model.Classification{}, fmt.Errorf("failed to write direction prompt: %w", err)
		}
		if _, err := fmt.Fprintln(p.writer, "  [I] Income - money coming in"); err != nil {
			return model.Classification{}, fmt.Errorf("failed to write income option: %w", err)
		}
		if _, err := fmt.Fprintln(p.writer, "  [E] Expense - money going out"); err != nil {
			return model.Classification{}, fmt.Errorf("failed to write expense option: %w", err)
		}
		if _, err := fmt.Fprintln(p.writer, "  [T] Transfer - between accounts"); err != nil {
			return model.Classification{}, fmt.Errorf("failed to write transfer option: %w", err)
		}
		if _, err := fmt.Fprintln(p.writer); err != nil {
			return model.Classification{}, fmt.Errorf("failed to write newline: %w", err)
		}

		dirChoice, err := p.promptChoice(ctx, "Direction", []string{"i", "e", "t"})
		if err != nil {
			return model.Classification{}, err
		}

		switch dirChoice {
		case "i":
			direction = model.DirectionIncome
		case "e":
			direction = model.DirectionExpense
		case "t":
			direction = model.DirectionTransfer
		}

		if _, err := fmt.Fprintln(p.writer); err != nil {
			return model.Classification{}, fmt.Errorf("failed to write newline: %w", err)
		}
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
	if pending.DirectionConfidence >= 0.8 && pending.SuggestedDirection != "" {
		// Only show direction change option if we didn't already prompt for it
		if _, err := fmt.Fprintln(p.writer, "  [D] Change transaction direction"); err != nil {
			return model.Classification{}, fmt.Errorf("failed to write change direction option: %w", err)
		}
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

	if pending.DirectionConfidence >= 0.8 && pending.SuggestedDirection != "" {
		validChoices = append(validChoices, "d")
	}

	choice, err := p.promptChoice(ctx, "Choice", validChoices)
	if err != nil {
		return model.Classification{}, err
	}

	// Update transaction direction
	pending.Transaction.Direction = direction

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
			category, err := p.promptCategorySelection(ctx, pending.CategoryRankings, pending.CheckPatterns)
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
		category, err := p.promptCategorySelection(ctx, pending.CategoryRankings, pending.CheckPatterns)
		if err != nil {
			return model.Classification{}, err
		}
		classification.Category = category
		classification.Status = model.StatusUserModified
		p.trackCategorization(pending.Transaction.MerchantName, category)
		p.incrementStats(true, false)
	case "d":
		// Change direction
		if _, err := fmt.Fprintln(p.writer); err != nil {
			return model.Classification{}, fmt.Errorf("failed to write newline: %w", err)
		}
		if _, err := fmt.Fprintln(p.writer, FormatPrompt("Select new direction:")); err != nil {
			return model.Classification{}, fmt.Errorf("failed to write direction prompt: %w", err)
		}
		if _, err := fmt.Fprintln(p.writer, "  [I] Income - money coming in"); err != nil {
			return model.Classification{}, fmt.Errorf("failed to write income option: %w", err)
		}
		if _, err := fmt.Fprintln(p.writer, "  [E] Expense - money going out"); err != nil {
			return model.Classification{}, fmt.Errorf("failed to write expense option: %w", err)
		}
		if _, err := fmt.Fprintln(p.writer, "  [T] Transfer - between accounts"); err != nil {
			return model.Classification{}, fmt.Errorf("failed to write transfer option: %w", err)
		}
		if _, err := fmt.Fprintln(p.writer); err != nil {
			return model.Classification{}, fmt.Errorf("failed to write newline: %w", err)
		}

		dirChoice, err := p.promptChoice(ctx, "Direction", []string{"i", "e", "t"})
		if err != nil {
			return model.Classification{}, err
		}

		switch dirChoice {
		case "i":
			pending.Transaction.Direction = model.DirectionIncome
		case "e":
			pending.Transaction.Direction = model.DirectionExpense
		case "t":
			pending.Transaction.Direction = model.DirectionTransfer
		}

		// Re-run the classification with the new direction
		return p.ConfirmClassification(ctx, pending)
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

	// Add spacing after progress bar
	if _, err := fmt.Fprintln(p.writer); err != nil {
		slog.Warn("Failed to write newline", "error", err)
	}

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

	return nil, fmt.Errorf("invalid selection '%s'. Please choose from the available options", choice)
}

// ConfirmTransactionDirection prompts the user to confirm or select a transaction direction.
func (p *Prompter) ConfirmTransactionDirection(ctx context.Context, pending engine.PendingDirection) (model.TransactionDirection, error) {
	// Display header
	header := fmt.Sprintf("Direction Detection - %s", pending.MerchantName)
	if _, err := fmt.Fprintf(p.writer, "\n%s\n", FormatTitle(header)); err != nil {
		return "", fmt.Errorf("failed to write header: %w", err)
	}

	// Show transaction details
	if _, err := fmt.Fprintf(p.writer, "Found %d transaction(s) from this merchant\n", pending.TransactionCount); err != nil {
		return "", fmt.Errorf("failed to write transaction count: %w", err)
	}

	// Show sample transaction
	if _, err := fmt.Fprintf(p.writer, "\nSample transaction:\n"); err != nil {
		return "", fmt.Errorf("failed to write sample header: %w", err)
	}
	if _, err := fmt.Fprintf(p.writer, "  Description: %s\n", pending.SampleTransaction.Name); err != nil {
		return "", fmt.Errorf("failed to write description: %w", err)
	}
	if _, err := fmt.Fprintf(p.writer, "  Amount: $%.2f\n", pending.SampleTransaction.Amount); err != nil {
		return "", fmt.Errorf("failed to write amount: %w", err)
	}
	if _, err := fmt.Fprintf(p.writer, "  Date: %s\n", pending.SampleTransaction.Date.Format("Jan 2, 2006")); err != nil {
		return "", fmt.Errorf("failed to write date: %w", err)
	}
	if pending.SampleTransaction.Type != "" {
		if _, err := fmt.Fprintf(p.writer, "  Type: %s\n", pending.SampleTransaction.Type); err != nil {
			return "", fmt.Errorf("failed to write type: %w", err)
		}
	}

	// Show AI suggestion
	suggestionText := fmt.Sprintf(
		"Suggested: %s (%.0f%% confidence)\nReasoning: %s",
		getDirectionDisplay(pending.SuggestedDirection),
		pending.Confidence*100,
		pending.Reasoning,
	)
	if _, err := fmt.Fprintf(p.writer, "\n%s\n", FormatInfo(suggestionText)); err != nil {
		return "", fmt.Errorf("failed to write AI suggestion: %w", err)
	}

	// Show options
	if _, err := fmt.Fprintln(p.writer, "\nOptions:"); err != nil {
		return "", fmt.Errorf("failed to write options header: %w", err)
	}
	if _, err := fmt.Fprintln(p.writer, "  [1] Income"); err != nil {
		return "", fmt.Errorf("failed to write income option: %w", err)
	}
	if _, err := fmt.Fprintln(p.writer, "  [2] Expense"); err != nil {
		return "", fmt.Errorf("failed to write expense option: %w", err)
	}
	if _, err := fmt.Fprintln(p.writer, "  [3] Transfer"); err != nil {
		return "", fmt.Errorf("failed to write transfer option: %w", err)
	}
	if _, err := fmt.Fprintf(p.writer, "  [A] Accept AI suggestion (%s)\n", getDirectionDisplay(pending.SuggestedDirection)); err != nil {
		return "", fmt.Errorf("failed to write accept option: %w", err)
	}
	if _, err := fmt.Fprintln(p.writer); err != nil {
		return "", fmt.Errorf("failed to write newline: %w", err)
	}

	// Get user choice
	choice, err := p.promptChoice(ctx, "Choice [1/2/3/A]", []string{"1", "2", "3", "a"})
	if err != nil {
		return "", err
	}

	// Map choice to direction
	switch choice {
	case "1":
		return model.DirectionIncome, nil
	case "2":
		return model.DirectionExpense, nil
	case "3":
		return model.DirectionTransfer, nil
	case "a":
		return pending.SuggestedDirection, nil
	default:
		return "", fmt.Errorf("invalid choice: %s", choice)
	}
}

// getDirectionDisplay returns a user-friendly display string for a direction.
func getDirectionDisplay(direction model.TransactionDirection) string {
	switch direction {
	case model.DirectionIncome:
		return "Income"
	case model.DirectionExpense:
		return "Expense"
	case model.DirectionTransfer:
		return "Transfer"
	default:
		return string(direction)
	}
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

	// Add direction information
	var directionIcon string
	switch pending.SuggestedDirection {
	case model.DirectionIncome:
		directionIcon = "📈"
	case model.DirectionExpense:
		directionIcon = "📉"
	case model.DirectionTransfer:
		directionIcon = "➡️"
	default:
		directionIcon = "❓"
	}

	details += fmt.Sprintf("\n%s Direction: %s %s (%.0f%% confidence)",
		InfoIcon, directionIcon, string(pending.SuggestedDirection), pending.DirectionConfidence*100)
	if pending.DirectionReasoning != "" {
		details += fmt.Sprintf("\n  %s %s", InfoIcon, pending.DirectionReasoning)
	}

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

	if pending.IsCategoryMismatch {
		suggestion += fmt.Sprintf("\n  %s Warning: Category type mismatch! %s is for %s transactions",
			WarningIcon, pending.SuggestedCategory, "income/expense") // This will be filled by actual category type
	}

	if pending.SimilarCount > 0 {
		suggestion += fmt.Sprintf("\n  %s Similar transactions: %d", InfoIcon, pending.SimilarCount)
	}

	return header + "\n\n" + details + suggestion
}

func (p *Prompter) formatBatchSummary(pending []model.PendingClassification, pattern string) string {
	merchantName := pending[0].Transaction.MerchantName
	suggestedCategory := pending[0].SuggestedCategory
	suggestedDirection := pending[0].SuggestedDirection

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

	// Add direction icon
	var directionIcon string
	switch suggestedDirection {
	case model.DirectionIncome:
		directionIcon = "📈"
	case model.DirectionExpense:
		directionIcon = "📉"
	case model.DirectionTransfer:
		directionIcon = "➡️"
	default:
		directionIcon = "❓"
	}

	summary += fmt.Sprintf("  Direction: %s %s\n", directionIcon, string(suggestedDirection))

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

		choice, err := p.reader.ReadLine(ctx)
		if err != nil {
			if err == ErrInputCancelled {
				return "", context.Canceled
			}
			if err == io.EOF {
				return "", fmt.Errorf("input canceled by user")
			}
			return "", err
		}

		choice = strings.ToLower(choice)

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

func (p *Prompter) promptCategorySelection(ctx context.Context, rankings model.CategoryRankings, checkPatterns []model.CheckPattern) (string, error) {
	if _, err := fmt.Fprintln(p.writer); err != nil {
		return "", fmt.Errorf("failed to write newline: %w", err)
	}

	if _, err := fmt.Fprintln(p.writer, FormatPrompt("Select category (ranked by likelihood):")); err != nil {
		return "", fmt.Errorf("failed to write selection header: %w", err)
	}
	if _, err := fmt.Fprintln(p.writer); err != nil {
		return "", fmt.Errorf("failed to write newline: %w", err)
	}

	// Build category number map and display options
	categoryMap := make(map[string]string)  // number -> category name
	categoryByName := make(map[string]bool) // for name-based selection

	// Display ranked categories
	for i, ranking := range rankings {
		if i >= 15 { // Limit display to top 15 categories
			break
		}

		num := fmt.Sprintf("%d", i+1)
		categoryMap[num] = ranking.Category
		categoryByName[strings.ToLower(ranking.Category)] = true

		// Build the display line
		var line string
		if ranking.Score >= 0.01 { // Only show percentage if >= 1%
			line = fmt.Sprintf("  [%s] %s (%.0f%% match)",
				num, ranking.Category, ranking.Score*100)
		} else {
			line = fmt.Sprintf("  [%s] %s", num, ranking.Category)
		}

		// Add check pattern indicator
		for _, pattern := range checkPatterns {
			if pattern.Category == ranking.Category && pattern.Active {
				line += fmt.Sprintf(" %s matches pattern \"%s\"",
					SuccessStyle.Render("⭐"), pattern.PatternName)
				break
			}
		}

		if _, err := fmt.Fprintln(p.writer, line); err != nil {
			return "", fmt.Errorf("failed to write category option: %w", err)
		}

		// Show description if available
		if ranking.Description != "" {
			if _, err := fmt.Fprintf(p.writer, "      %s\n",
				SubtleStyle.Render(ranking.Description)); err != nil {
				return "", fmt.Errorf("failed to write category description: %w", err)
			}
		}

		if _, err := fmt.Fprintln(p.writer); err != nil {
			return "", fmt.Errorf("failed to write spacing: %w", err)
		}
	}

	// Add option to show more categories if there are more than 15
	if len(rankings) > 15 {
		if _, err := fmt.Fprintf(p.writer, "  [M] Show %d more categories\n",
			len(rankings)-15); err != nil {
			return "", fmt.Errorf("failed to write show more option: %w", err)
		}
		if _, err := fmt.Fprintln(p.writer); err != nil {
			return "", fmt.Errorf("failed to write newline: %w", err)
		}
	}

	// Add new category option
	if _, err := fmt.Fprintln(p.writer, "  [N] Create new category"); err != nil {
		return "", fmt.Errorf("failed to write new category option: %w", err)
	}
	if _, err := fmt.Fprintln(p.writer); err != nil {
		return "", fmt.Errorf("failed to write newline: %w", err)
	}

	showingAll := false

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		if _, err := fmt.Fprint(p.writer, FormatPrompt("Enter number or category name: ")); err != nil {
			return "", fmt.Errorf("failed to write selection prompt: %w", err)
		}

		choice, err := p.reader.ReadLine(ctx)
		if err != nil {
			if err == ErrInputCancelled {
				return "", context.Canceled
			}
			return "", err
		}
		if choice == "" {
			if _, err := fmt.Fprintln(p.writer, FormatError("Please make a selection.")); err != nil {
				slog.Warn("Failed to write empty selection error", "error", err)
			}
			continue
		}

		// Handle special options
		lowerChoice := strings.ToLower(choice)
		if lowerChoice == "m" && len(rankings) > 15 && !showingAll {
			// Show all categories
			showingAll = true
			if _, err := fmt.Fprintln(p.writer); err != nil {
				return "", fmt.Errorf("failed to write newline: %w", err)
			}

			// Display remaining categories
			for i := 15; i < len(rankings); i++ {
				ranking := rankings[i]
				num := fmt.Sprintf("%d", i+1)
				categoryMap[num] = ranking.Category
				categoryByName[strings.ToLower(ranking.Category)] = true

				var line string
				if ranking.Score >= 0.01 {
					line = fmt.Sprintf("  [%s] %s (%.0f%% match)",
						num, ranking.Category, ranking.Score*100)
				} else {
					line = fmt.Sprintf("  [%s] %s", num, ranking.Category)
				}

				if _, err := fmt.Fprintln(p.writer, line); err != nil {
					return "", fmt.Errorf("failed to write category option: %w", err)
				}
			}

			if _, err := fmt.Fprintln(p.writer); err != nil {
				return "", fmt.Errorf("failed to write newline: %w", err)
			}
			continue
		}

		if lowerChoice == "n" {
			// Prompt for new category name
			return p.promptNewCategoryName(ctx)
		}

		// Check if it's a number selection
		if category, ok := categoryMap[choice]; ok {
			return category, nil
		}

		// Check if it's a category name (case-insensitive)
		if categoryByName[lowerChoice] {
			// Find the exact category name
			for _, ranking := range rankings {
				if strings.ToLower(ranking.Category) == lowerChoice {
					return ranking.Category, nil
				}
			}
		}

		if _, err := fmt.Fprintln(p.writer, FormatError("Invalid selection. Please enter a number, category name, or 'N' for new category.")); err != nil {
			slog.Warn("Failed to write invalid selection error", "error", err)
		}
	}
}

func (p *Prompter) promptNewCategoryName(ctx context.Context) (string, error) {
	if _, err := fmt.Fprintln(p.writer); err != nil {
		return "", fmt.Errorf("failed to write newline: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		if _, err := fmt.Fprint(p.writer, FormatPrompt("Enter new category name: ")); err != nil {
			return "", fmt.Errorf("failed to write new category prompt: %w", err)
		}

		category, err := p.reader.ReadLine(ctx)
		if err != nil {
			if err == ErrInputCancelled {
				return "", context.Canceled
			}
			return "", err
		}
		if category == "" {
			if _, err := fmt.Fprintln(p.writer, FormatError("Category name cannot be empty. Please try again.")); err != nil {
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
		// Set direction on transaction
		pc.Transaction.Direction = pc.SuggestedDirection

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
	// Use rankings from the first transaction (they should all be similar for a merchant group)
	var rankings model.CategoryRankings
	var checkPatterns []model.CheckPattern
	if len(pending) > 0 {
		rankings = pending[0].CategoryRankings
		checkPatterns = pending[0].CheckPatterns
	}

	category, err := p.promptCategorySelection(ctx, rankings, checkPatterns)
	if err != nil {
		return nil, err
	}

	classifications := make([]model.Classification, len(pending))

	for i, pc := range pending {
		// Set direction on transaction
		pc.Transaction.Direction = pc.SuggestedDirection

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
