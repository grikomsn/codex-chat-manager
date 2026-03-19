package session

import (
	"fmt"
	"path/filepath"
	"time"
)

// Status identifies whether a rollout file lives in the active or archived tree.
type Status string

const (
	// StatusActive marks a rollout file in ~/.codex/sessions.
	StatusActive Status = "active"
	// StatusArchived marks a rollout file in ~/.codex/archived_sessions.
	StatusArchived Status = "archived"
	// StatusMixed marks a group that contains both active and archived descendants.
	StatusMixed Status = "mixed"
)

// SessionRecord is the canonical representation of one rollout JSONL file.
type SessionRecord struct {
	ID            string    `json:"id"`
	Path          string    `json:"path"`
	Status        Status    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	CWD           string    `json:"cwd,omitempty"`
	Title         string    `json:"title,omitempty"`
	Source        string    `json:"source,omitempty"`
	AgentNickname string    `json:"agent_nickname,omitempty"`
	AgentRole     string    `json:"agent_role,omitempty"`
	ParentID      string    `json:"parent_id,omitempty"`
	ChildCount    int       `json:"child_count"`
	SizeBytes     int64     `json:"size_bytes"`
	IsOrphan      bool      `json:"is_orphan,omitempty"`
	HasPreview    bool      `json:"has_preview"`
}

// DisplayTitle returns the best human-facing title for a session.
func (s SessionRecord) DisplayTitle() string {
	if s.Title != "" {
		return s.Title
	}
	if s.CWD != "" {
		return filepath.Base(s.CWD)
	}
	return s.ID
}

// Subtitle returns a concise secondary descriptor.
func (s SessionRecord) Subtitle() string {
	base := filepath.Base(s.CWD)
	if base == "." || base == string(filepath.Separator) {
		base = s.CWD
	}
	if base == "" {
		base = "unknown cwd"
	}
	switch {
	case s.AgentNickname != "":
		return fmt.Sprintf("%s (%s)", base, s.AgentNickname)
	case s.AgentRole != "":
		return fmt.Sprintf("%s (%s)", base, s.AgentRole)
	default:
		return base
	}
}

// SessionGroup is the top-level row shown to users.
type SessionGroup struct {
	Parent       SessionRecord   `json:"parent"`
	Children     []SessionRecord `json:"children,omitempty"`
	Status       Status          `json:"status"`
	AggregateAt  time.Time       `json:"aggregate_at"`
	MixedStatus  bool            `json:"mixed_status"`
	ChildCount   int             `json:"child_count"`
	CascadesTo   []string        `json:"cascades_to"`
	ParentExists bool            `json:"parent_exists"`
}

// PreviewBlockKind identifies one rendered unit in a preview document.
type PreviewBlockKind string

const (
	PreviewUser      PreviewBlockKind = "user"
	PreviewAssistant PreviewBlockKind = "assistant"
	PreviewToolCall  PreviewBlockKind = "tool_call"
	PreviewToolOut   PreviewBlockKind = "tool_output"
	PreviewEvent     PreviewBlockKind = "event"
)

// PreviewBlock is a logical transcript fragment that can be rendered at any width.
type PreviewBlock struct {
	Kind  PreviewBlockKind `json:"kind"`
	Title string           `json:"title,omitempty"`
	Body  string           `json:"body,omitempty"`
}

// PreviewDocument is the lazily parsed transcript preview for one session.
type PreviewDocument struct {
	SessionID string         `json:"session_id"`
	Title     string         `json:"title"`
	Blocks    []PreviewBlock `json:"blocks"`
}

// ActionType identifies a supported filesystem mutation.
type ActionType string

const (
	ActionArchive   ActionType = "archive"
	ActionUnarchive ActionType = "unarchive"
	ActionDelete    ActionType = "delete"
)

// ActionTarget is one concrete file mutation target.
type ActionTarget struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	Status     Status `json:"status"`
	ParentID   string `json:"parent_id,omitempty"`
	IsChild    bool   `json:"is_child"`
	IsSelected bool   `json:"is_selected"`
}

// ActionSkip captures a non-fatal reason an item was not changed.
type ActionSkip struct {
	ID     string `json:"id,omitempty"`
	Path   string `json:"path,omitempty"`
	Reason string `json:"reason"`
}

// ActionPlan describes a resolved mutation and its side effects.
type ActionPlan struct {
	Type               ActionType     `json:"type"`
	RequestedIDs       []string       `json:"requested_ids"`
	TargetIDs          []string       `json:"target_ids"`
	Targets            []ActionTarget `json:"targets"`
	Skipped            []ActionSkip   `json:"skipped,omitempty"`
	RemovedIndexRows   int            `json:"removed_index_rows,omitempty"`
	RemovedSnapshots   []string       `json:"removed_snapshots,omitempty"`
	BlockedByActiveIDs []string       `json:"blocked_by_active_ids,omitempty"`
}
