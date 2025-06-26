package llm

import (
	"strings"
)

// cleanMarkdownWrapper removes markdown code block wrappers from content.
// LLMs often wrap JSON responses in ```json ... ``` blocks.
func cleanMarkdownWrapper(content string) string {
	content = strings.TrimSpace(content)

	// Check for JSON-specific code blocks first
	if strings.HasPrefix(content, "```json") && strings.HasSuffix(content, "```") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
		return strings.TrimSpace(content)
	}

	// Check for generic code blocks
	if strings.HasPrefix(content, "```") && strings.HasSuffix(content, "```") {
		// Find the end of the first line to skip any language identifier
		firstNewline := strings.Index(content, "\n")
		if firstNewline > 0 {
			content = content[firstNewline+1:]
		} else {
			content = strings.TrimPrefix(content, "```")
		}
		content = strings.TrimSuffix(content, "```")
		return strings.TrimSpace(content)
	}

	// Extract JSON from content that may have explanatory text before it
	// Look for the start of a JSON object or array
	jsonStart := strings.Index(content, "{")
	jsonArrayStart := strings.Index(content, "[")

	// Use whichever comes first
	if jsonStart == -1 || (jsonArrayStart != -1 && jsonArrayStart < jsonStart) {
		jsonStart = jsonArrayStart
	}

	if jsonStart > 0 {
		// Found JSON after some text, extract from that point
		content = content[jsonStart:]

		// Find the matching closing brace/bracket
		braceCount := 0
		bracketCount := 0
		inString := false
		escapeNext := false

		for i, ch := range content {
			if escapeNext {
				escapeNext = false
				continue
			}

			if ch == '\\' {
				escapeNext = true
				continue
			}

			if ch == '"' {
				inString = !inString
				continue
			}

			if inString {
				continue
			}

			switch ch {
			case '{':
				braceCount++
			case '}':
				braceCount--
			case '[':
				bracketCount++
			case ']':
				bracketCount--
			}

			// Check if we've closed all braces/brackets
			if braceCount == 0 && bracketCount == 0 && i > 0 {
				return content[:i+1]
			}
		}
	}

	return content
}
