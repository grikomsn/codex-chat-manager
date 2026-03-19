package session

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	EnvCodexHome           = "CODEX_HOME"
	sessionsSubdir         = "sessions"
	archivedSessionsSubdir = "archived_sessions"
	sessionIndexName       = "session_index.jsonl"
	shellSnapshotsSubdir   = "shell_snapshots"
)

// Config contains the resolved Codex storage paths.
type Config struct {
	CodexHome        string
	SessionsDir      string
	ArchivedDir      string
	SessionIndexPath string
	ShellSnapshots   string
}

// ResolveConfig expands and normalizes the Codex storage root.
func ResolveConfig(override string) (Config, error) {
	root := override
	if root == "" {
		root = os.Getenv(EnvCodexHome)
	}
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Config{}, fmt.Errorf("resolve home directory: %w", err)
		}
		root = filepath.Join(home, ".codex")
	}
	root = filepath.Clean(root)
	return Config{
		CodexHome:        root,
		SessionsDir:      filepath.Join(root, sessionsSubdir),
		ArchivedDir:      filepath.Join(root, archivedSessionsSubdir),
		SessionIndexPath: filepath.Join(root, sessionIndexName),
		ShellSnapshots:   filepath.Join(root, shellSnapshotsSubdir),
	}, nil
}
