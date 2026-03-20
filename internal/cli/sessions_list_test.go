package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grikomsn/codex-chat-manager/internal/session"
)

func TestSessionsListJSONShapes(t *testing.T) {
	root := t.TempDir()
	baseTime := time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC)

	writeRolloutFixture(t, root, fixtureSpec{
		dir:        "sessions",
		datePath:   "2026/03/19",
		id:         "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		cwd:        "/tmp/active",
		title:      "Active Session",
		statusTime: baseTime.Add(1 * time.Minute),
	})
	writeRolloutFixture(t, root, fixtureSpec{
		dir:        "archived_sessions",
		id:         "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		cwd:        "/tmp/archived",
		title:      "Archived Session",
		statusTime: baseTime.Add(2 * time.Minute),
	})
	writeRolloutFixture(t, root, fixtureSpec{
		dir:        "sessions",
		datePath:   "2026/03/19",
		id:         "cccccccc-cccc-cccc-cccc-cccccccccccc",
		cwd:        "/tmp/mixed-parent",
		title:      "Mixed Parent",
		statusTime: baseTime.Add(3 * time.Minute),
	})
	writeRolloutFixture(t, root, fixtureSpec{
		dir:        "archived_sessions",
		id:         "dddddddd-dddd-dddd-dddd-dddddddddddd",
		cwd:        "/tmp/mixed-child",
		title:      "Mixed Child",
		parentID:   "cccccccc-cccc-cccc-cccc-cccccccccccc",
		statusTime: baseTime.Add(4 * time.Minute),
	})
	writeRolloutFixture(t, root, fixtureSpec{
		dir:        "sessions",
		datePath:   "2026/03/19",
		id:         "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee",
		cwd:        "/tmp/orphan-child",
		title:      "Orphan Child",
		parentID:   "ffffffff-ffff-ffff-ffff-ffffffffffff",
		statusTime: baseTime.Add(5 * time.Minute),
	})

	stdout, _, err := executeCLI(t, root, "sessions", "list", "--json")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	envelope := decodeEnvelope(t, stdout)
	if !envelope.OK {
		t.Fatalf("expected ok response, got %s", stdout)
	}

	var groups []session.SessionGroup
	if err := json.Unmarshal(envelope.Data, &groups); err != nil {
		t.Fatalf("unmarshal groups: %v", err)
	}
	if len(groups) != 4 {
		t.Fatalf("expected 4 groups, got %d", len(groups))
	}

	byID := make(map[string]session.SessionGroup, len(groups))
	for _, group := range groups {
		byID[group.Parent.ID] = group
	}

	active := byID["aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"]
	if active.Status != session.StatusActive || !active.ParentExists || active.ChildCount != 0 {
		t.Fatalf("unexpected active group: %+v", active)
	}

	archived := byID["bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"]
	if archived.Status != session.StatusArchived || !archived.ParentExists || archived.ChildCount != 0 {
		t.Fatalf("unexpected archived group: %+v", archived)
	}

	mixed := byID["cccccccc-cccc-cccc-cccc-cccccccccccc"]
	if mixed.Status != session.StatusMixed || !mixed.ParentExists || mixed.ChildCount != 1 {
		t.Fatalf("unexpected mixed group: %+v", mixed)
	}
	if len(mixed.Children) != 1 || mixed.Children[0].Status != session.StatusArchived {
		t.Fatalf("unexpected mixed children: %+v", mixed.Children)
	}

	orphan := byID["ffffffff-ffff-ffff-ffff-ffffffffffff"]
	if orphan.Status != session.StatusActive || orphan.ParentExists {
		t.Fatalf("unexpected orphan group: %+v", orphan)
	}
	if !orphan.Parent.IsOrphan {
		t.Fatalf("expected orphan parent flag: %+v", orphan.Parent)
	}
	if len(orphan.Children) != 1 || orphan.Children[0].ID != "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee" {
		t.Fatalf("unexpected orphan children: %+v", orphan.Children)
	}
}

type fixtureSpec struct {
	dir        string
	datePath   string
	id         string
	cwd        string
	title      string
	parentID   string
	statusTime time.Time
}

func writeRolloutFixture(t *testing.T, root string, spec fixtureSpec) {
	t.Helper()

	dir := filepath.Join(root, spec.dir)
	if spec.datePath != "" {
		dir = filepath.Join(dir, filepath.FromSlash(spec.datePath))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir fixture dir: %v", err)
	}

	source := `"source":"vscode"`
	if spec.parentID != "" {
		source = `"source":{"subagent":{"thread_spawn":{"parent_thread_id":"` + spec.parentID + `"}}}`
	}

	body := `{"type":"session-meta","payload":{"id":"` + spec.id + `","cwd":"` + spec.cwd + `",` + source + `}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"user_message","message":"` + spec.title + `"}}` + "\n"

	filename := "rollout-" + spec.statusTime.UTC().Format("2006-01-02T15-04-05") + "-" + spec.id + ".jsonl"
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.Chtimes(path, spec.statusTime, spec.statusTime); err != nil {
		t.Fatalf("chtimes fixture: %v", err)
	}
}
