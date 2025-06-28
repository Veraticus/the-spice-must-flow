package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
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
	} else {
		if _, err := fmt.Fprintf(p.writer, "  [A] Accept AI suggestion: %s\n", SuccessStyle.Render(pending.SuggestedCategory)); err != nil {
			return model.Classification{}, fmt.Errorf("failed to write AI suggestion: %w", err)
		}
	}

	if _, err := fmt.Fprintln(p.writer, "  [E] Select category"); err != nil {
		return model.Classification{}, fmt.Errorf("failed to write select category option: %w", err)
	}
	if _, err := fmt.Fprintln(p.writer, "  [S] Skip this transaction"); err != nil {
		return model.Classification{}, fmt.Errorf("failed to write skip option: %w", err)
	}
	if _, err := fmt.Fprintln(p.writer); err != nil {
		return model.Classification{}, fmt.Errorf("failed to write newline: %w", err)
	}

	var validChoices = []string{"a", "e", "s"}

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
		// Show all categories for selection
		category, err := p.promptCategorySelection(ctx, pending.CategoryRankings, pending.AllCategories, pending.CheckPatterns)
		if err != nil {
			return model.Classification{}, err
		}
		classification.Category = category
		classification.Status = model.StatusUserModified
		classification.Confidence = 1.0
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

	// Don't update progress here - wait until we know what the user chose

	merchantName := pending[0].Transaction.MerchantName
	pattern := p.detectPattern(merchantName)

	content := p.formatBatchSummary(pending, pattern)
	if _, err := fmt.Fprintln(p.writer, RenderBox("Batch Review", content)); err != nil {
		// Log write errors but continue - don't fail the entire batch due to display issues
		slog.Warn("Failed to write batch review box", "error", err, "merchant", merchantName)
	}

	if _, err := fmt.Fprintln(p.writer, FormatPrompt("Options:")); err != nil {
		slog.Warn("Failed to write options prompt", "error", err)
	}

	if pending[0].IsNewCategory {
		if _, err := fmt.Fprintf(p.writer, "  [A] Create and use new category '%s' for all %d transactions\n",
			pending[0].SuggestedCategory, len(pending)); err != nil {
			slog.Warn("Failed to write new category accept option", "error", err)
		}
	} else {
		if _, err := fmt.Fprintf(p.writer, "  [A] Accept for all %d transactions\n", len(pending)); err != nil {
			slog.Warn("Failed to write batch accept option", "error", err)
		}
	}

	if _, err := fmt.Fprintln(p.writer, "  [E] Select category for all"); err != nil {
		slog.Warn("Failed to write select category option", "error", err)
	}
	if _, err := fmt.Fprintln(p.writer, "  [R] Review each transaction individually"); err != nil {
		slog.Warn("Failed to write review option", "error", err)
	}
	if _, err := fmt.Fprintln(p.writer, "  [S] Skip all transactions"); err != nil {
		slog.Warn("Failed to write skip all option", "error", err)
	}
	if _, err := fmt.Fprintln(p.writer); err != nil {
		slog.Warn("Failed to write newline", "error", err)
	}

	var validChoices = []string{"a", "e", "r", "s"}
	var promptText = "Choice [A/E/R/S]"

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
		// Select category for all transactions
		return p.customCategoryForAll(ctx, pending)
	case "r":
		return p.reviewEachTransaction(ctx, pending)
	case "s":
		return p.skipAllClassifications(pending)
	}

	return nil, fmt.Errorf("invalid selection '%s'. Please choose from the available options", choice)
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

	type completionJSON struct {
		Duration          string  `json:"duration"`
		TimeSaved         string  `json:"time_saved"`
		TotalTransactions int     `json:"total_transactions"`
		AutoClassified    int     `json:"auto_classified"`
		AutoClassifiedPct float64 `json:"auto_classified_percent"`
		UserClassified    int     `json:"user_classified"`
		NewVendorRules    int     `json:"new_vendor_rules"`
	}

	data := completionJSON{
		TotalTransactions: stats.TotalTransactions,
		AutoClassified:    stats.AutoClassified,
		AutoClassifiedPct: float64(stats.AutoClassified) / float64(stats.TotalTransactions) * 100,
		UserClassified:    stats.UserClassified,
		NewVendorRules:    stats.NewVendorRules,
		Duration:          stats.Duration.Round(time.Second).String(),
		TimeSaved:         timeSaved,
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		slog.Warn("Failed to marshal completion stats", "error", err)
		return
	}

	if _, err := fmt.Fprintln(p.writer, string(bytes)); err != nil {
		slog.Warn("Failed to write completion stats", "error", err)
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
	p.updateProgressBy(1)
}

func (p *Prompter) updateProgressBy(count int) {
	p.processedCount += count
	if p.progressBar != nil {
		if err := p.progressBar.Add(count); err != nil {
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
				return "", fmt.Errorf("input canceled by user")
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

func (p *Prompter) promptCategorySelection(ctx context.Context, rankings model.CategoryRankings, allCategories []model.Category, checkPatterns []model.CheckPattern) (string, error) {
	if _, err := fmt.Fprintln(p.writer); err != nil {
		return "", fmt.Errorf("failed to write newline: %w", err)
	}

	if _, err := fmt.Fprintln(p.writer, FormatPrompt("Select category:")); err != nil {
		return "", fmt.Errorf("failed to write selection header: %w", err)
	}
	if _, err := fmt.Fprintln(p.writer); err != nil {
		return "", fmt.Errorf("failed to write newline: %w", err)
	}

	// Build category number map and display options
	categoryMap := make(map[string]string)  // number -> category name
	categoryByName := make(map[string]bool) // for name-based selection

	// Create a map of rankings for quick lookup
	rankingScores := make(map[string]float64)
	for _, ranking := range rankings {
		rankingScores[ranking.Category] = ranking.Score
	}

	// Merge rankings with all categories
	type categoryDisplay struct {
		name        string
		description string
		score       float64
		isNew       bool
	}

	var displayCategories []categoryDisplay
	seenCategories := make(map[string]bool)

	// First add ranked categories
	for _, ranking := range rankings {
		if !ranking.IsNew {
			displayCategories = append(displayCategories, categoryDisplay{
				name:        ranking.Category,
				score:       ranking.Score,
				isNew:       false,
				description: ranking.Description,
			})
			seenCategories[ranking.Category] = true
		}
	}

	// Then add unranked categories from allCategories
	for _, cat := range allCategories {
		if !seenCategories[cat.Name] {
			displayCategories = append(displayCategories, categoryDisplay{
				name:        cat.Name,
				score:       0.0,
				isNew:       false,
				description: cat.Description,
			})
			seenCategories[cat.Name] = true
		}
	}

	// Sort: high scores first, then alphabetically
	sort.Slice(displayCategories, func(i, j int) bool {
		if displayCategories[i].score != displayCategories[j].score {
			return displayCategories[i].score > displayCategories[j].score
		}
		return displayCategories[i].name < displayCategories[j].name
	})

	// Display categories
	displayCount := 0
	for i, cat := range displayCategories {
		if displayCount >= 15 && cat.score < 0.01 { // Show max 15 unless they have significant scores
			break
		}

		num := fmt.Sprintf("%d", i+1)
		categoryMap[num] = cat.name
		categoryByName[strings.ToLower(cat.name)] = true

		// Build the display line
		var line string
		if cat.score >= 0.01 { // Only show percentage if >= 1%
			line = fmt.Sprintf("  [%s] %s (%.0f%% match)",
				num, cat.name, cat.score*100)
		} else {
			line = fmt.Sprintf("  [%s] %s", num, cat.name)
		}

		displayCount++

		// Add check pattern indicator
		for _, pattern := range checkPatterns {
			if pattern.Category == cat.name {
				line += fmt.Sprintf(" %s matches pattern \"%s\"",
					SuccessStyle.Render("⭐"), pattern.PatternName)
				break
			}
		}

		if _, err := fmt.Fprintln(p.writer, line); err != nil {
			return "", fmt.Errorf("failed to write category option: %w", err)
		}

		// Show description if available
		if cat.description != "" && cat.score >= 0.01 {
			if _, err := fmt.Fprintf(p.writer, "      %s\n",
				SubtleStyle.Render(cat.description)); err != nil {
				return "", fmt.Errorf("failed to write category description: %w", err)
			}
		}

		if _, err := fmt.Fprintln(p.writer); err != nil {
			return "", fmt.Errorf("failed to write spacing: %w", err)
		}
	}

	// Add option to show more categories if there are more
	if displayCount < len(displayCategories) {
		if _, err := fmt.Fprintf(p.writer, "  [M] Show %d more categories\n",
			len(displayCategories)-displayCount); err != nil {
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

		input, err := p.reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		choice := strings.TrimSpace(input)
		if choice == "" {
			if _, err := fmt.Fprintln(p.writer, FormatError("Please make a selection.")); err != nil {
				slog.Warn("Failed to write empty selection error", "error", err)
			}
			continue
		}

		// Handle special options
		lowerChoice := strings.ToLower(choice)
		if lowerChoice == "m" && displayCount < len(displayCategories) && !showingAll {
			// Show all categories
			showingAll = true
			if _, err := fmt.Fprintln(p.writer); err != nil {
				return "", fmt.Errorf("failed to write newline: %w", err)
			}

			// Display remaining categories
			for i := displayCount; i < len(displayCategories); i++ {
				cat := displayCategories[i]
				num := fmt.Sprintf("%d", i+1)
				categoryMap[num] = cat.name
				categoryByName[strings.ToLower(cat.name)] = true

				var line string
				if cat.score >= 0.01 {
					line = fmt.Sprintf("  [%s] %s (%.0f%% match)",
						num, cat.name, cat.score*100)
				} else {
					line = fmt.Sprintf("  [%s] %s", num, cat.name)
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

	// First, get the category name
	var categoryName string
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		if _, err := fmt.Fprint(p.writer, FormatPrompt("Enter new category name: ")); err != nil {
			return "", fmt.Errorf("failed to write new category prompt: %w", err)
		}

		input, err := p.reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		categoryName = strings.TrimSpace(input)
		if categoryName == "" {
			if _, err := fmt.Fprintln(p.writer, FormatError("Category name cannot be empty. Please try again.")); err != nil {
				slog.Warn("Failed to write empty category error", "error", err)
			}
			continue
		}
		break
	}

	// Then, ask about description
	if _, err := fmt.Fprintln(p.writer); err != nil {
		return "", fmt.Errorf("failed to write newline: %w", err)
	}

	if _, err := fmt.Fprintln(p.writer, FormatInfo("Would you like to add a description for this category?")); err != nil {
		return "", fmt.Errorf("failed to write description prompt: %w", err)
	}
	if _, err := fmt.Fprintln(p.writer, "  [Y] Yes, I'll provide a description"); err != nil {
		return "", fmt.Errorf("failed to write yes option: %w", err)
	}
	if _, err := fmt.Fprintln(p.writer, "  [N] No, let AI generate one"); err != nil {
		return "", fmt.Errorf("failed to write no option: %w", err)
	}
	if _, err := fmt.Fprintln(p.writer); err != nil {
		return "", fmt.Errorf("failed to write newline: %w", err)
	}

	choice, err := p.promptChoice(ctx, "Choice [Y/N]", []string{"y", "n"})
	if err != nil {
		return "", err
	}

	if choice == "y" {
		// Get description from user
		if _, err := fmt.Fprint(p.writer, FormatPrompt("Enter description: ")); err != nil {
			return "", fmt.Errorf("failed to write description prompt: %w", err)
		}

		input, err := p.reader.ReadString('\n')
		if err != nil {
			return "", err
		}

		description := strings.TrimSpace(input)
		// Return category name with description marker
		// We'll use a special format to indicate it has a description
		return categoryName + "|DESC|" + description, nil
	}

	// Return just the category name, AI will generate description
	return categoryName, nil
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
	p.updateProgressBy(len(pending)) // Update progress by batch size
	if _, err := fmt.Fprintln(p.writer, FormatSuccess(fmt.Sprintf("✓ Classified %d transactions as %s",
		len(pending), pending[0].SuggestedCategory))); err != nil {
		slog.Warn("Failed to write success message", "error", err)
	}

	return classifications, nil
}

func (p *Prompter) customCategoryForAll(ctx context.Context, pending []model.PendingClassification) ([]model.Classification, error) {
	// Use rankings from the first transaction (they should all be similar for a merchant group)
	var rankings model.CategoryRankings
	var allCategories []model.Category
	var checkPatterns []model.CheckPattern
	if len(pending) > 0 {
		rankings = pending[0].CategoryRankings
		allCategories = pending[0].AllCategories
		checkPatterns = pending[0].CheckPatterns
	}

	category, err := p.promptCategorySelection(ctx, rankings, allCategories, checkPatterns)
	if err != nil {
		return nil, err
	}

	// Check if this is a new category with description
	var categoryName string
	var categoryDescription string
	var isNewCategory bool

	if strings.Contains(category, "|DESC|") {
		parts := strings.Split(category, "|DESC|")
		categoryName = parts[0]
		if len(parts) > 1 {
			categoryDescription = parts[1]
		}
		isNewCategory = true
	} else {
		categoryName = category
		// Check if this category already exists
		categoryExists := false
		for _, cat := range allCategories {
			if cat.Name == categoryName {
				categoryExists = true
				break
			}
		}
		isNewCategory = !categoryExists

		// Additional debug logging
		if isNewCategory {
			slog.Debug("Category not found in existing categories",
				"category", categoryName,
				"existingCategoriesCount", len(allCategories))
		}
	}

	classifications := make([]model.Classification, len(pending))

	for i, pc := range pending {
		classifications[i] = model.Classification{
			Transaction:  pc.Transaction,
			Category:     categoryName,
			Status:       model.StatusUserModified,
			Confidence:   1.0,
			ClassifiedAt: time.Now(),
		}
		// Store metadata to indicate this is a new category that needs creation
		if isNewCategory && i == 0 {
			// Use notes field to pass the new category info
			// This is a temporary way to signal the engine about the new category
			if categoryDescription != "" {
				classifications[i].Notes = fmt.Sprintf("NEW_CATEGORY|%s", categoryDescription)
			} else {
				classifications[i].Notes = "NEW_CATEGORY|"
			}
			slog.Debug("Setting new category signal",
				"category", categoryName,
				"description", categoryDescription,
				"notes", classifications[i].Notes,
				"isNewCategory", isNewCategory)
		}
		p.trackCategorization(pc.Transaction.MerchantName, categoryName)
	}

	p.incrementBatchStats(len(pending), true, true)
	p.updateProgressBy(len(pending)) // Update progress by batch size
	if _, err := fmt.Fprintln(p.writer, FormatSuccess(fmt.Sprintf("✓ Classified %d transactions as %s",
		len(pending), categoryName))); err != nil {
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
			Category:     "", // Explicitly set empty category for unclassified status
			Status:       model.StatusUnclassified,
			ClassifiedAt: time.Now(),
		}
	}

	p.updateProgressBy(len(pending)) // Update progress for skipped transactions
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
