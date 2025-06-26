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

	return content
}
