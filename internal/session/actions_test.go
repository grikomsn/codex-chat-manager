package session

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDeleteBlocksActiveSessions(t *testing.T) {
	t.Parallel()
	store, parentID := testStoreWithOneSession(t, StatusActive)
	plan, err := store.Delete([]string{parentID})
	if err == nil {
		t.Fatal("expected delete to be blocked for active session")
	}
	if !errors.Is(err, ErrDeleteBlockedActive) {
		t.Fatalf("expected ErrDeleteBlockedActive, got %v", err)
	}
	var blockedErr *DeleteBlockedActiveError
	if !errors.As(err, &blockedErr) {
		t.Fatalf("expected DeleteBlockedActiveError, got %T", err)
	}
	if len(blockedErr.ActiveIDs) != 1 || blockedErr.ActiveIDs[0] != parentID {
		t.Fatalf("unexpected blocked ids: %+v", blockedErr.ActiveIDs)
	}
	if len(plan.BlockedByActiveIDs) != 1 || plan.BlockedByActiveIDs[0] != parentID {
		t.Fatalf("unexpected blocked ids in plan: %+v", plan.BlockedByActiveIDs)
	}
	if len(plan.Targets) != 0 {
		t.Fatalf("expected no delete targets, got %d", len(plan.Targets))
	}
	if _, statErr := os.Stat(filepath.Join(store.cfg.SessionsDir, "2026", "03", "19", "rollout-2026-03-19T10-42-03-"+parentID+".jsonl")); statErr != nil {
		t.Fatalf("expected active rollout to remain on disk, stat err = %v", statErr)
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

func TestArchiveSkipsUnknownIDs(t *testing.T) {
	t.Parallel()
	store, id := testStoreWithOneSession(t, StatusActive)

	plan, err := store.Archive([]string{id, "missing"})
	if err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	if len(plan.TargetIDs) != 1 || plan.TargetIDs[0] != id {
		t.Fatalf("unexpected archive targets: %+v", plan.TargetIDs)
	}
	if len(plan.Skipped) != 1 || plan.Skipped[0].ID != "missing" || plan.Skipped[0].Reason != "not found" {
		t.Fatalf("unexpected skipped entries: %+v", plan.Skipped)
	}
}

func TestDeleteSkipsUnknownIDs(t *testing.T) {
	t.Parallel()
	store, id := testStoreWithOneSession(t, StatusArchived)

	plan, err := store.Delete([]string{id, "missing"})
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if len(plan.TargetIDs) != 1 || plan.TargetIDs[0] != id {
		t.Fatalf("unexpected delete targets: %+v", plan.TargetIDs)
	}
	if len(plan.Skipped) != 1 || plan.Skipped[0].ID != "missing" || plan.Skipped[0].Reason != "not found" {
		t.Fatalf("unexpected skipped entries: %+v", plan.Skipped)
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

func TestResumeIntentForActiveSession(t *testing.T) {
	t.Parallel()

	store, id := testStoreWithOneSessionAndCWD(t, StatusActive, t.TempDir())

	intent, err := store.ResumeIntent(id)
	if err != nil {
		t.Fatalf("ResumeIntent() error = %v", err)
	}
	if !intent.Eligible {
		t.Fatalf("expected eligible resume intent")
	}
	if intent.Status != StatusActive {
		t.Fatalf("expected active status, got %q", intent.Status)
	}
	if intent.WorkingDirectory == "" {
		t.Fatal("expected working directory")
	}
	if intent.Executable != "codex" {
		t.Fatalf("expected codex executable, got %q", intent.Executable)
	}
	if len(intent.Args) != 2 || intent.Args[0] != "resume" || intent.Args[1] != id {
		t.Fatalf("unexpected args: %+v", intent.Args)
	}
	if intent.EnvOverrides[EnvCodexHome] != store.cfg.CodexHome {
		t.Fatalf("expected CODEX_HOME override %q, got %q", store.cfg.CodexHome, intent.EnvOverrides[EnvCodexHome])
	}
}

func TestResumeIntentRejectsArchivedSession(t *testing.T) {
	t.Parallel()

	store, id := testStoreWithOneSessionAndCWD(t, StatusArchived, t.TempDir())

	intent, err := store.ResumeIntent(id)
	if err == nil {
		t.Fatal("expected archived resume error")
	}
	if !errors.Is(err, ErrResumeIneligible) {
		t.Fatalf("expected ErrResumeIneligible, got %v", err)
	}
	if intent.SessionID != id {
		t.Fatalf("expected session id %q, got %q", id, intent.SessionID)
	}
	if intent.Eligible {
		t.Fatal("expected ineligible intent")
	}
	if intent.Status != StatusArchived {
		t.Fatalf("expected archived status, got %q", intent.Status)
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

func testStoreWithOneSessionAndCWD(t *testing.T, status Status, cwd string) (*Store, string) {
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
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-2026-03-19T10-42-03-"+id+".jsonl")
	body := `{"type":"session_meta","payload":{"id":"` + id + `","cwd":"` + cwd + `","source":"vscode"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"user_message","message":"title"}}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return NewStore(cfg), id
}
