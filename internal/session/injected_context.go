package session

import "strings"

const injectedAgentsContextPreviewLimit = 2500

func isInjectedAgentsContext(message string) bool {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return false
	}

	firstLine := trimmed
	if idx := strings.IndexByte(firstLine, '\n'); idx >= 0 {
		firstLine = firstLine[:idx]
	}
	firstLine = strings.TrimSpace(firstLine)

	lower := strings.ToLower(trimmed)
	lowerFirst := strings.ToLower(firstLine)
	mentionsAgents := strings.HasPrefix(lowerFirst, "# agents.md") ||
		strings.HasPrefix(lowerFirst, "agents.md") ||
		strings.Contains(lowerFirst, "agents.md instructions for")
	if !mentionsAgents {
		return false
	}

	hasMarkers := strings.Contains(lower, "<instructions>") || strings.Contains(lower, "<environment_context>")
	return hasMarkers
}
