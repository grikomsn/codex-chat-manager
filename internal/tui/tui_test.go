package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/grikomsn/codex-chat-manager/internal/session"
)

func TestInitialModelLoadsSessionAndResizesWide(t *testing.T) {
	t.Parallel()
	cfg := makeTUIFixture(t)
	m, err := initialModel(cfg)
	if err != nil {
		t.Fatalf("initialModel() error = %v", err)
	}
	if len(m.groups) != 1 {
		t.Fatalf("expected one group, got %d", len(m.groups))
	}
	m.width = 140
	m.height = 40
	m.resize()
	if !m.isWide() {
		t.Fatal("expected wide mode")
	}
}

func TestInitialModelResizesNarrow(t *testing.T) {
	t.Parallel()
	cfg := makeTUIFixture(t)
	m, err := initialModel(cfg)
	if err != nil {
		t.Fatalf("initialModel() error = %v", err)
	}
	m.width = 100
	m.height = 30
	m.resize()
	if m.isWide() {
		t.Fatal("expected narrow mode")
	}
}

func TestSyncPreviewHidesSystemInstructionsByDefault(t *testing.T) {
	t.Parallel()
	cfg := makeTUIFixtureWithContext(t)
	m, err := initialModel(cfg)
	if err != nil {
		t.Fatalf("initialModel() error = %v", err)
	}
	m.width = 140
	m.height = 40
	m.resize()
	group := m.groups[0]
	m.current = &group
	m.syncPreview()
	if strings.Contains(m.viewport.View(), "developer system instructions") {
		t.Fatalf("expected hidden system instructions, got %q", m.viewport.View())
	}
	m.showSystem = true
	m.syncPreview()
	if !strings.Contains(m.viewport.View(), "developer system instructions") {
		t.Fatalf("expected visible system instructions, got %q", m.viewport.View())
	}
}

func makeTUIFixture(t *testing.T) session.Config {
	t.Helper()
	root := t.TempDir()
	cfg := session.Config{
		CodexHome:        root,
		SessionsDir:      filepath.Join(root, "sessions"),
		ArchivedDir:      filepath.Join(root, "archived_sessions"),
		SessionIndexPath: filepath.Join(root, "session_index.jsonl"),
		ShellSnapshots:   filepath.Join(root, "shell_snapshots"),
	}
	path := filepath.Join(cfg.SessionsDir, "2026", "03", "19")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"session_meta","payload":{"id":"11111111-1111-1111-1111-111111111111","cwd":"/tmp/app","source":"vscode"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"user_message","message":"test title"}}` + "\n"
	if err := os.WriteFile(filepath.Join(path, "rollout-2026-03-19T10-42-03-11111111-1111-1111-1111-111111111111.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func makeTUIFixtureWithContext(t *testing.T) session.Config {
	t.Helper()
	root := t.TempDir()
	cfg := session.Config{
		CodexHome:        root,
		SessionsDir:      filepath.Join(root, "sessions"),
		ArchivedDir:      filepath.Join(root, "archived_sessions"),
		SessionIndexPath: filepath.Join(root, "session_index.jsonl"),
		ShellSnapshots:   filepath.Join(root, "shell_snapshots"),
	}
	path := filepath.Join(cfg.SessionsDir, "2026", "03", "19")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"session_meta","payload":{"id":"11111111-1111-1111-1111-111111111111","cwd":"/tmp/app","source":"vscode"}}` + "\n" +
		`{"type":"response_item","payload":{"type":"message","role":"developer","content":[{"type":"input_text","text":"developer system instructions"}]}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"user_message","message":"test title"}}` + "\n"
	if err := os.WriteFile(filepath.Join(path, "rollout-2026-03-19T10-42-03-11111111-1111-1111-1111-111111111111.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfg
}
