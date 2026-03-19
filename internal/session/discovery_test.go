package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseRolloutName(t *testing.T) {
	t.Parallel()
	gotTime, gotID, err := parseRolloutName("rollout-2026-03-19T10-42-03-019d042f-a040-71c0-9dc7-aefe4db66a2a.jsonl")
	if err != nil {
		t.Fatalf("parseRolloutName() error = %v", err)
	}
	if gotID != "019d042f-a040-71c0-9dc7-aefe4db66a2a" {
		t.Fatalf("unexpected id %q", gotID)
	}
	want := time.Date(2026, 3, 19, 10, 42, 3, 0, time.UTC)
	if !gotTime.Equal(want) {
		t.Fatalf("unexpected time %v", gotTime)
	}
}

func TestLoadSnapshotIgnoresNoiseAndBuildsGroups(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cfg := Config{
		CodexHome:        root,
		SessionsDir:      filepath.Join(root, "sessions"),
		ArchivedDir:      filepath.Join(root, "archived_sessions"),
		SessionIndexPath: filepath.Join(root, "session_index.jsonl"),
		ShellSnapshots:   filepath.Join(root, "shell_snapshots"),
	}

	activePath := filepath.Join(cfg.SessionsDir, "2026", "03", "19")
	if err := os.MkdirAll(activePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.SessionsDir, ".DS_Store"), []byte("noise"), 0o644); err != nil {
		t.Fatal(err)
	}
	parentID := "11111111-1111-1111-1111-111111111111"
	childID := "22222222-2222-2222-2222-222222222222"
	parent := filepath.Join(activePath, "rollout-2026-03-19T10-42-03-"+parentID+".jsonl")
	child := filepath.Join(activePath, "rollout-2026-03-19T10-43-03-"+childID+".jsonl")
	parentBody := `{"type":"session_meta","payload":{"id":"` + parentID + `","cwd":"/tmp/app","source":"vscode"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"user_message","message":"parent title"}}` + "\n"
	childBody := `{"type":"session_meta","payload":{"id":"` + childID + `","cwd":"/tmp/app","source":{"subagent":{"thread_spawn":{"parent_thread_id":"` + parentID + `","agent_nickname":"Faraday","agent_role":"explorer"}}}}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"user_message","message":"child title"}}` + "\n"
	if err := os.WriteFile(parent, []byte(parentBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(child, []byte(childBody), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewStore(cfg)
	snapshot, err := store.LoadSnapshot()
	if err != nil {
		t.Fatalf("LoadSnapshot() error = %v", err)
	}
	if len(snapshot.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(snapshot.Groups))
	}
	group := snapshot.Groups[0]
	if group.Parent.ID != parentID {
		t.Fatalf("unexpected parent id %q", group.Parent.ID)
	}
	if len(group.Children) != 1 || group.Children[0].ID != childID {
		t.Fatalf("unexpected children %#v", group.Children)
	}
}
