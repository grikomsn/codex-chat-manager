package session

import "strings"

const (
	StatusFilterAll      = "all"
	StatusFilterActive   = "active"
	StatusFilterArchived = "archived"
)

// FilterGroups filters groups by status and free-text match.
func FilterGroups(groups []SessionGroup, statusFilter, text string, includeChildren bool) []SessionGroup {
	statusFilter = strings.ToLower(strings.TrimSpace(statusFilter))
	text = strings.ToLower(strings.TrimSpace(text))
	if statusFilter == "" {
		statusFilter = StatusFilterAll
	}
	filtered := make([]SessionGroup, 0, len(groups))
	for _, group := range groups {
		if statusFilter != StatusFilterAll {
			switch statusFilter {
			case string(StatusActive), string(StatusArchived):
				if string(group.Status) != statusFilter && group.Status != StatusMixed {
					continue
				}
			default:
				if string(group.Status) != statusFilter {
					continue
				}
			}
		}
		if text != "" && !groupContains(group, text, includeChildren) {
			continue
		}
		filtered = append(filtered, group)
	}
	return filtered
}

func groupContains(group SessionGroup, text string, includeChildren bool) bool {
	fields := []string{
		group.Parent.ID,
		group.Parent.DisplayTitle(),
		group.Parent.CWD,
		group.Parent.Source,
		group.Parent.AgentNickname,
		group.Parent.AgentRole,
	}
	if includeChildren {
		for _, child := range group.Children {
			fields = append(fields, child.ID, child.DisplayTitle(), child.CWD, child.AgentNickname, child.AgentRole)
		}
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), text) {
			return true
		}
	}
	return false
}
