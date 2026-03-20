package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDeleteRemovesActiveSessions(t *testing.T) {
	t.Parallel()
	store, parentID := testStoreWithOneSession(t, StatusActive)
	plan, err := store.Delete([]string{parentID})
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if len(plan.Targets) != 1 {
		t.Fatalf("expected 1 delete target, got %d", len(plan.Targets))
	}
	if _, statErr := os.Stat(plan.Targets[0].Path); !os.IsNotExist(statErr) {
		t.Fatalf("expected active rollout to be removed, stat err = %v", statErr)
	}
}

func TestArchiveAndUnarchiveMoveFiles(t *testing.T) {
	t.Parallel()
	store, id := testStoreWithOneSession(t, StatusActive)
	plan, err := store.Archive([]string{id})
	if err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	if len(plan.Targets) != 1 {
		t.Fatalf("expected 1 archive target, got %d", len(plan.Targets))
	}
	snapshot, err := store.LoadSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	record := snapshot.RecordsByID[id]
	before := record.UpdatedAt
	time.Sleep(10 * time.Millisecond)
	plan, err = store.Unarchive([]string{id})
	if err != nil {
		t.Fatalf("Unarchive() error = %v", err)
	}
	if len(plan.Targets) != 1 {
		t.Fatalf("expected 1 unarchive target, got %d", len(plan.Targets))
	}
	snapshot, err = store.LoadSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	record = snapshot.RecordsByID[id]
	if !record.UpdatedAt.After(before) {
		t.Fatalf("expected updated mtime after unarchive")
	}
}

func TestDeleteRemovesIndexAndSnapshot(t *testing.T) {
	t.Parallel()
	store, id := testStoreWithOneSession(t, StatusArchived)
	if err := os.MkdirAll(store.cfg.ShellSnapshots, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.cfg.ShellSnapshots, id+".sh"), []byte("echo hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.cfg.SessionIndexPath, []byte(`{"id":"`+id+`","thread_name":"x","updated_at":"2026-01-01T00:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := store.Delete([]string{id})
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if plan.RemovedIndexRows != 1 {
		t.Fatalf("expected 1 removed index row, got %d", plan.RemovedIndexRows)
	}
	if len(plan.RemovedSnapshots) != 1 {
		t.Fatalf("expected 1 removed snapshot, got %d", len(plan.RemovedSnapshots))
	}
}

func TestResolveTargetsCascadesParent(t *testing.T) {
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
	parentID := "11111111-1111-1111-1111-111111111111"
	childID := "22222222-2222-2222-2222-222222222222"
	parent := filepath.Join(activePath, "rollout-2026-03-19T10-42-03-"+parentID+".jsonl")
	child := filepath.Join(activePath, "rollout-2026-03-19T10-43-03-"+childID+".jsonl")
	if err := os.WriteFile(parent, []byte(`{"type":"session_meta","payload":{"id":"`+parentID+`","cwd":"/tmp/app","source":"vscode"}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(child, []byte(`{"type":"session_meta","payload":{"id":"`+childID+`","cwd":"/tmp/app","source":{"subagent":{"thread_spawn":{"parent_thread_id":"`+parentID+`","agent_nickname":"Faraday","agent_role":"explorer"}}}}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(cfg)
	_, records, err := store.ResolveTargets([]string{parentID})
	if err != nil {
		t.Fatalf("ResolveTargets() error = %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected cascade to child, got %d records", len(records))
	}
}

func testStoreWithOneSession(t *testing.T, status Status) (*Store, string) {
	t.Helper()
	root := t.TempDir()
	cfg := Config{
		CodexHome:        root,
		SessionsDir:      filepath.Join(root, "sessions"),
		ArchivedDir:      filepath.Join(root, "archived_sessions"),
		SessionIndexPath: filepath.Join(root, "session_index.jsonl"),
		ShellSnapshots:   filepath.Join(root, "shell_snapshots"),
	}
	id := "11111111-1111-1111-1111-111111111111"
	dir := filepath.Join(cfg.SessionsDir, "2026", "03", "19")
	if status == StatusArchived {
		dir = cfg.ArchivedDir
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-2026-03-19T10-42-03-"+id+".jsonl")
	body := `{"type":"session_meta","payload":{"id":"` + id + `","cwd":"/tmp/app","source":"vscode"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"user_message","message":"title"}}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return NewStore(cfg), id
}
