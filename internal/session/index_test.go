package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadIndexLatestWins(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "session_index.jsonl")
	body := "" +
		`{"id":"one","thread_name":"first","updated_at":"2026-03-01T00:00:00Z"}` + "\n" +
		`{"id":"one","thread_name":"second","updated_at":"2026-03-02T00:00:00Z"}` + "\n" +
		`{"id":"two","thread_name":"other","updated_at":"2026-03-03T00:00:00Z"}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	state, err := loadIndex(path)
	if err != nil {
		t.Fatalf("loadIndex() error = %v", err)
	}
	if state.Titles["one"] != "second" {
		t.Fatalf("expected latest title, got %q", state.Titles["one"])
	}
	if len(state.Lines) != 3 {
		t.Fatalf("expected 3 raw lines, got %d", len(state.Lines))
	}
}

func TestRewriteIndexRemovesMatchingRows(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "session_index.jsonl")
	body := "" +
		`{"id":"one","thread_name":"first","updated_at":"2026-03-01T00:00:00Z"}` + "\n" +
		`{"id":"two","thread_name":"other","updated_at":"2026-03-03T00:00:00Z"}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	removed, err := rewriteIndex(path, map[string]struct{}{"one": {}})
	if err != nil {
		t.Fatalf("rewriteIndex() error = %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed row, got %d", removed)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"id":"two","thread_name":"other","updated_at":"2026-03-03T00:00:00Z"}`+"\n" {
		t.Fatalf("unexpected file contents %q", string(data))
	}
}
