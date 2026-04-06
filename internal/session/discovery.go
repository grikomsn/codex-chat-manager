package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var rolloutNameRE = regexp.MustCompile(`^rollout-(\d{4})-(\d{2})-(\d{2})T(\d{2})-(\d{2})-(\d{2})-([0-9a-f-]+)\.jsonl$`)

type rolloutMeta struct {
	id            string
	cwd           string
	source        string
	parentID      string
	agentNickname string
	agentRole     string
	titleFallback string
	hasPreview    bool
}

type recordEnvelope struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

type sessionMetaPayload struct {
	ID            string          `json:"id"`
	CWD           string          `json:"cwd"`
	ModelProvider string          `json:"model_provider"`
	Source        json.RawMessage `json:"source"`
}

type outputMessagePayload struct {
	Type    string          `json:"type"`
	Role    string          `json:"role"`
	Content []messagePart   `json:"content"`
	Message string          `json:"message"`
	Args    json.RawMessage `json:"arguments"`
	Input   json.RawMessage `json:"input"`
	CallID  string          `json:"call_id"`
	Name    string          `json:"name"`
	Status  string          `json:"status"`
	Output  string          `json:"output"`
}

type eventPayload struct {
	Type    string          `json:"type"`
	Message string          `json:"message"`
	Params  json.RawMessage `json:"params"`
}

type messagePart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type sourceString struct {
	Subagent *struct {
		ThreadSpawn *struct {
			ParentThreadID string `json:"parent_thread_id"`
			AgentNickname  string `json:"agent_nickname"`
			AgentRole      string `json:"agent_role"`
		} `json:"thread_spawn"`
	} `json:"subagent"`
}

// Snapshot is the fully resolved in-memory catalog.
type Snapshot struct {
	RecordsByID map[string]SessionRecord
	Groups      []SessionGroup
}

func loadSnapshotImpl(cfg Config) (Snapshot, error) {
	slog.Debug("loading snapshot", "sessions_dir", cfg.SessionsDir, "archived_dir", cfg.ArchivedDir)
	index, err := loadIndex(cfg.SessionIndexPath)
	if err != nil {
		return Snapshot{}, err
	}

	var records []SessionRecord
	active, err := scanRoot(cfg.SessionsDir, StatusActive, index.Titles)
	if err != nil {
		return Snapshot{}, err
	}
	archived, err := scanRoot(cfg.ArchivedDir, StatusArchived, index.Titles)
	if err != nil {
		return Snapshot{}, err
	}
	records = append(records, active...)
	records = append(records, archived...)
	slog.Debug("snapshot records loaded", "active", len(active), "archived", len(archived), "total", len(records))

	resolver := newProjectResolver()
	for i := range records {
		key, label := resolver.Resolve(records[i].CWD)
		records[i].ProjectKey = key
		records[i].Project = label
	}

	byID := make(map[string]SessionRecord, len(records))
	for _, record := range records {
		byID[record.ID] = record
	}

	groups := groupRecords(records, byID)
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].AggregateAt.Equal(groups[j].AggregateAt) {
			return groups[i].Parent.ID > groups[j].Parent.ID
		}
		return groups[i].AggregateAt.After(groups[j].AggregateAt)
	})

	return Snapshot{
		RecordsByID: byID,
		Groups:      groups,
	}, nil
}

func scanRoot(root string, status Status, titles map[string]string) ([]SessionRecord, error) {
	slog.Debug("scanning directory", "path", root, "status", status)
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("directory does not exist", "path", root)
			return nil, nil
		}
		return nil, fmt.Errorf("stat %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, nil
	}

	records := make([]SessionRecord, 0, 128)
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		record, err := readRecord(path, status, titles)
		if err != nil {
			slog.Warn("skip unreadable rollout", "path", path, "error", err)
			return nil
		}
		records = append(records, record)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", root, err)
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].UpdatedAt.Equal(records[j].UpdatedAt) {
			return records[i].ID > records[j].ID
		}
		return records[i].UpdatedAt.After(records[j].UpdatedAt)
	})
	slog.Debug("scan completed", "path", root, "status", status, "records", len(records))
	return records, nil
}

func readRecord(path string, status Status, titles map[string]string) (SessionRecord, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return SessionRecord{}, err
	}
	createdAt, id, err := parseRolloutName(filepath.Base(path))
	if err != nil {
		return SessionRecord{}, err
	}
	meta, err := readRolloutHead(path)
	if err != nil {
		return SessionRecord{}, err
	}
	if meta.id != "" {
		id = meta.id
	}
	title := titles[id]
	if title == "" {
		title = meta.titleFallback
	}
	return SessionRecord{
		ID:            id,
		Path:          path,
		Status:        status,
		CreatedAt:     createdAt,
		UpdatedAt:     stat.ModTime().UTC(),
		CWD:           meta.cwd,
		Title:         cleanTitle(title),
		Source:        meta.source,
		AgentNickname: meta.agentNickname,
		AgentRole:     meta.agentRole,
		ParentID:      meta.parentID,
		SizeBytes:     stat.Size(),
		HasPreview:    meta.hasPreview,
	}, nil
}

func parseRolloutName(name string) (time.Time, string, error) {
	matches := rolloutNameRE.FindStringSubmatch(name)
	if matches == nil {
		return time.Time{}, "", fmt.Errorf("invalid rollout filename: %s", name)
	}
	createdAt, err := time.Parse("2006-01-02 15:04:05", fmt.Sprintf("%s-%s-%s %s:%s:%s", matches[1], matches[2], matches[3], matches[4], matches[5], matches[6]))
	if err != nil {
		return time.Time{}, "", fmt.Errorf("parse rollout filename: %w", err)
	}
	return createdAt.UTC(), matches[7], nil
}

func readRolloutHead(path string) (rolloutMeta, error) {
	file, err := os.Open(path)
	if err != nil {
		return rolloutMeta{}, fmt.Errorf("open rollout: %w", err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	var meta rolloutMeta
	linesRead := 0
	for linesRead < 250 {
		line, err := reader.ReadBytes('\n')
		if err != nil && err != io.EOF {
			return meta, fmt.Errorf("read rollout head: %w", err)
		}
		trimmed := strings.TrimSpace(string(line))
		if trimmed != "" {
			linesRead++
			var env recordEnvelope
			if json.Unmarshal([]byte(trimmed), &env) == nil {
				switch env.Type {
				case RecordTypeSessionMeta, RecordTypeSessionMetaHyphen:
					if meta.id == "" {
						readSessionMeta(&meta, env.Payload)
					}
				case RecordTypeEventMsg, RecordTypeEventMsgHyphen:
					readEventTitle(&meta, env.Payload)
				case RecordTypeResponseItem, RecordTypeResponseItemHyphen:
					readResponseMessage(&meta, env.Payload)
				}
			}
		}
		if meta.id != "" && meta.hasPreview && meta.titleFallback != "" {
			break
		}
		if err == io.EOF {
			break
		}
	}
	return meta, nil
}

func readSessionMeta(meta *rolloutMeta, raw json.RawMessage) {
	var payload sessionMetaPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}
	meta.id = payload.ID
	meta.cwd = payload.CWD
	meta.source = decodeSourceSummary(payload.Source)
	meta.parentID, meta.agentNickname, meta.agentRole = decodeSourceDetails(payload.Source)
}

func decodeSourceSummary(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return ""
	}
	if _, ok := data["subagent"]; ok {
		return "subagent"
	}
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func decodeSourceDetails(raw json.RawMessage) (string, string, string) {
	var source sourceString
	if err := json.Unmarshal(raw, &source); err != nil {
		return "", "", ""
	}
	if source.Subagent == nil || source.Subagent.ThreadSpawn == nil {
		return "", "", ""
	}
	return source.Subagent.ThreadSpawn.ParentThreadID, source.Subagent.ThreadSpawn.AgentNickname, source.Subagent.ThreadSpawn.AgentRole
}

func readEventTitle(meta *rolloutMeta, raw json.RawMessage) {
	var payload eventPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}
	if payload.Type == "user_message" && meta.titleFallback == "" {
		if isInjectedAgentsContext(payload.Message) {
			return
		}
		meta.titleFallback = payload.Message
		meta.hasPreview = payload.Message != ""
	}
}

func readResponseMessage(meta *rolloutMeta, raw json.RawMessage) {
	var payload outputMessagePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}
	if payload.Type != "message" {
		return
	}
	if payload.Role == "developer" || payload.Role == "system" {
		return
	}
	var chunks []string
	for _, part := range payload.Content {
		if part.Text != "" {
			chunks = append(chunks, part.Text)
		}
	}
	joined := strings.Join(chunks, " ")
	joined = strings.TrimSpace(joined)
	if joined == "" {
		return
	}
	meta.hasPreview = true
	if meta.titleFallback == "" {
		meta.titleFallback = joined
	}
}

func cleanTitle(s string) string {
	s = strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
	if len(s) > 120 {
		return s[:117] + "..."
	}
	return s
}

func groupRecords(records []SessionRecord, byID map[string]SessionRecord) []SessionGroup {
	childrenByParent := make(map[string][]SessionRecord)
	topLevel := make([]SessionRecord, 0, len(records))
	for _, record := range records {
		if record.ParentID != "" {
			childrenByParent[record.ParentID] = append(childrenByParent[record.ParentID], record)
			continue
		}
		topLevel = append(topLevel, record)
	}

	groups := make([]SessionGroup, 0, len(topLevel)+len(childrenByParent))
	for _, parent := range topLevel {
		children := childrenByParent[parent.ID]
		sortChildren(children)
		group := buildGroup(parent, children, true)
		groups = append(groups, group)
		delete(childrenByParent, parent.ID)
	}
	for parentID, children := range childrenByParent {
		sortChildren(children)
		parent := children[0]
		parent.ID = parentID
		parent.ParentID = ""
		parent.IsOrphan = true
		parent.ChildCount = len(children)
		parent.Title = fmt.Sprintf("%s [orphan]", parent.DisplayTitle())
		group := buildGroup(parent, children, false)
		groups = append(groups, group)
	}
	return groups
}

func sortChildren(children []SessionRecord) {
	sort.Slice(children, func(i, j int) bool {
		if children[i].UpdatedAt.Equal(children[j].UpdatedAt) {
			return children[i].ID > children[j].ID
		}
		return children[i].UpdatedAt.After(children[j].UpdatedAt)
	})
}

func buildGroup(parent SessionRecord, children []SessionRecord, parentExists bool) SessionGroup {
	aggregate := parent.UpdatedAt
	statuses := map[Status]struct{}{parent.Status: {}}
	cascadesTo := []string{parent.ID}
	for _, child := range children {
		cascadesTo = append(cascadesTo, child.ID)
		statuses[child.Status] = struct{}{}
		if child.UpdatedAt.After(aggregate) {
			aggregate = child.UpdatedAt
		}
	}
	parent.ChildCount = len(children)
	groupStatus := parent.Status
	mixed := len(statuses) > 1
	if mixed {
		groupStatus = StatusMixed
	}
	return SessionGroup{
		Parent:       parent,
		Children:     children,
		Status:       groupStatus,
		AggregateAt:  aggregate,
		ChildCount:   len(children),
		CascadesTo:   cascadesTo,
		ParentExists: parentExists,
	}
}
