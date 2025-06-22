package llm

import (
	"fmt"
	"strconv"
	"strings"
)

// parseLLMRankings parses the ranking response from LLM into CategoryRanking slice.
func parseLLMRankings(content string) ([]CategoryRanking, error) {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	var rankings []CategoryRanking
	var inRankings bool
	var inNewCategory bool
	var newCategory CategoryRanking

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Check for section markers
		if line == "RANKINGS:" {
			inRankings = true
			inNewCategory = false
			continue
		}

		if line == "NEW_CATEGORY (only if needed):" || line == "NEW_CATEGORY:" {
			inRankings = false
			inNewCategory = true
			continue
		}

		// Parse rankings section
		if inRankings {
			parts := strings.Split(line, "|")
			if len(parts) != 2 {
				// Skip malformed lines but continue parsing
				continue
			}

			category := strings.TrimSpace(parts[0])
			scoreStr := strings.TrimSpace(parts[1])

			score, err := strconv.ParseFloat(scoreStr, 64)
			if err != nil {
				// Try to recover from common formatting issues
				// Check if it's a percentage (e.g., "85%")
				if strings.HasSuffix(scoreStr, "%") {
					percentStr := strings.TrimSuffix(scoreStr, "%")
					percentVal, parseErr := strconv.ParseFloat(strings.TrimSpace(percentStr), 64)
					if parseErr == nil {
						score = percentVal / 100.0
					} else {
						// Skip this ranking if we can't parse the score
						continue
					}
				} else {
					// Remove any non-numeric characters except decimal point
					cleanScore := strings.Map(func(r rune) rune {
						if (r >= '0' && r <= '9') || r == '.' {
							return r
						}
						return -1
					}, scoreStr)

					score, err = strconv.ParseFloat(cleanScore, 64)
					if err != nil {
						// Skip this ranking if we can't parse the score
						continue
					}
				}
			}

			// Validate score range
			if score < 0.0 {
				score = 0.0
			} else if score > 1.0 {
				score = 1.0
			}

			rankings = append(rankings, CategoryRanking{
				Category: category,
				Score:    score,
				IsNew:    false,
			})
		}

		// Parse new category section
		if inNewCategory {
			switch {
			case strings.HasPrefix(line, "name:"):
				newCategory.Category = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
				newCategory.IsNew = true
			case strings.HasPrefix(line, "score:"):
				scoreStr := strings.TrimSpace(strings.TrimPrefix(line, "score:"))
				score, err := strconv.ParseFloat(scoreStr, 64)
				if err == nil && score >= 0.0 && score <= 1.0 {
					newCategory.Score = score
				}
			case strings.HasPrefix(line, "description:"):
				newCategory.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			}
		}
	}

	// Add new category if it's valid
	if newCategory.Category != "" && newCategory.Score > 0 && newCategory.Description != "" {
		rankings = append(rankings, newCategory)
	}

	if len(rankings) == 0 {
		return nil, fmt.Errorf("no valid rankings found in response")
	}

	return rankings, nil
}

// parseClassificationWithRankings attempts to parse a response that may contain
// either the old format (CATEGORY/CONFIDENCE) or new format (RANKINGS).
func parseClassificationWithRankings(content string) ([]CategoryRanking, error) {
	// First try to parse as rankings format
	rankings, err := parseLLMRankings(content)
	if err == nil && len(rankings) > 0 {
		return rankings, nil
	}

	// Fall back to parsing old format and converting to rankings
	lines := strings.Split(strings.TrimSpace(content), "\n")
	var category string
	var confidence float64
	var isNew bool
	var description string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "CATEGORY:"):
			category = strings.TrimSpace(strings.TrimPrefix(line, "CATEGORY:"))
		case strings.HasPrefix(line, "CONFIDENCE:"):
			confStr := strings.TrimSpace(strings.TrimPrefix(line, "CONFIDENCE:"))
			confidence, _ = strconv.ParseFloat(confStr, 64)
		case strings.HasPrefix(line, "NEW:"):
			newStr := strings.TrimSpace(strings.TrimPrefix(line, "NEW:"))
			isNew = strings.ToLower(newStr) == "true"
		case strings.HasPrefix(line, "DESCRIPTION:"):
			description = strings.TrimSpace(strings.TrimPrefix(line, "DESCRIPTION:"))
		}
	}

	if category != "" && confidence > 0 {
		return []CategoryRanking{{
			Category:    category,
			Score:       confidence,
			IsNew:       isNew,
			Description: description,
		}}, nil
	}

	return nil, fmt.Errorf("unable to parse classification response")
}
