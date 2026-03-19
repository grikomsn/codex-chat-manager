package session

import (
	"fmt"
	"path/filepath"
	"time"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusArchived Status = "archived"
	StatusMixed    Status = "mixed"
)

const (
	RecordTypeSessionMeta        = "session_meta"
	RecordTypeSessionMetaHyphen  = "session-meta"
	RecordTypeEventMsg           = "event_msg"
	RecordTypeEventMsgHyphen     = "event-msg"
	RecordTypeResponseItem       = "response_item"
	RecordTypeResponseItemHyphen = "response-item"
)

type SessionRecord struct {
	ID            string    `json:"id"`
	Path          string    `json:"path"`
	Status        Status    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	CWD           string    `json:"cwd,omitempty"`
	Project       string    `json:"project,omitempty"`
	ProjectKey    string    `json:"-"`
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

func (s SessionRecord) DisplayTitle() string {
	if s.Title != "" {
		return s.Title
	}
	if s.CWD != "" {
		return filepath.Base(s.CWD)
	}
	return s.ID
}

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

type SessionGroup struct {
	Parent       SessionRecord   `json:"parent"`
	Children     []SessionRecord `json:"children,omitempty"`
	Status       Status          `json:"status"`
	AggregateAt  time.Time       `json:"aggregate_at"`
	ChildCount   int             `json:"child_count"`
	CascadesTo   []string        `json:"cascades_to"`
	ParentExists bool            `json:"parent_exists"`
}

func (g SessionGroup) HasChildren() bool {
	return len(g.Children) > 0
}

func (g SessionGroup) AllIDs() []string {
	return g.CascadesTo
}

func (g SessionGroup) IsActive() bool {
	return g.Status == StatusActive || g.Status == StatusMixed
}

func (r SessionRecord) IsArchived() bool {
	return r.Status == StatusArchived
}

type PreviewBlockKind string

const (
	PreviewUser      PreviewBlockKind = "user"
	PreviewAssistant PreviewBlockKind = "assistant"
	PreviewToolCall  PreviewBlockKind = "tool_call"
	PreviewToolOut   PreviewBlockKind = "tool_output"
	PreviewEvent     PreviewBlockKind = "event"
)

type PreviewBlock struct {
	Kind  PreviewBlockKind `json:"kind"`
	Title string           `json:"title,omitempty"`
	Body  string           `json:"body,omitempty"`
}

type PreviewDocument struct {
	SessionID string         `json:"session_id"`
	Title     string         `json:"title"`
	Blocks    []PreviewBlock `json:"blocks"`
}

type ActionType string

const (
	ActionArchive   ActionType = "archive"
	ActionUnarchive ActionType = "unarchive"
	ActionDelete    ActionType = "delete"
)

type ActionTarget struct {
	ID         string `json:"id"`
	Path       string `json:"path"`
	Status     Status `json:"status"`
	ParentID   string `json:"parent_id,omitempty"`
	IsChild    bool   `json:"is_child"`
	IsSelected bool   `json:"is_selected"`
}

type ActionSkip struct {
	ID     string `json:"id,omitempty"`
	Path   string `json:"path,omitempty"`
	Reason string `json:"reason"`
}

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
