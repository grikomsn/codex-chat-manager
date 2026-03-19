package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionsListJSON(t *testing.T) {
	root := t.TempDir()
	makeSessionFixture(t, root, "11111111-1111-1111-1111-111111111111", "Test title")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"sessions", "list", "--json", "--codex-home", root})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(stdout.String(), `"id": "11111111-1111-1111-1111-111111111111"`) {
		t.Fatalf("expected session id in output, got %s", stdout.String())
	}
}

func TestDeleteRequiresYes(t *testing.T) {
	root := t.TempDir()
	makeArchivedSessionFixture(t, root, "11111111-1111-1111-1111-111111111111")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"sessions", "delete", "--id", "11111111-1111-1111-1111-111111111111", "--codex-home", root})
	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected --yes error, got %v", err)
	}
}

func makeSessionFixture(t *testing.T, root, id, title string) {
	t.Helper()
	path := filepath.Join(root, "sessions", "2026", "03", "19")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"session-meta","payload":{"id":"` + id + `","cwd":"/tmp/app","source":"vscode"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"user_message","message":"` + title + `"}}` + "\n"
	if err := os.WriteFile(filepath.Join(path, "rollout-2026-03-19T10-42-03-"+id+".jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func makeArchivedSessionFixture(t *testing.T, root, id string) {
	t.Helper()
	path := filepath.Join(root, "archived_sessions")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"session-meta","payload":{"id":"` + id + `","cwd":"/tmp/app","source":"vscode"}}` + "\n"
	if err := os.WriteFile(filepath.Join(path, "rollout-2026-03-19T10-42-03-"+id+".jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
