package session

import (
	"os"
	"path/filepath"
	"strings"
)

type projectInfo struct {
	key   string
	label string
}

type gitRootCacheEntry struct {
	root  string
	found bool
}

type projectResolver struct {
	cwdCache map[string]projectInfo
	dirCache map[string]gitRootCacheEntry
}

func newProjectResolver() *projectResolver {
	return &projectResolver{
		cwdCache: make(map[string]projectInfo),
		dirCache: make(map[string]gitRootCacheEntry),
	}
}

func (r *projectResolver) Resolve(cwd string) (string, string) {
	cwd = strings.TrimSpace(cwd)
	if cwd == "" {
		return "unknown", "unknown"
	}
	if cached, ok := r.cwdCache[cwd]; ok {
		return cached.key, cached.label
	}

	// On non-Windows platforms, Windows-style paths are generally not stat-able.
	// Treat them as display-only and fall back to portable basename grouping.
	if looksLikeWindowsPath(cwd) {
		label := portableBase(cwd)
		if label == "" {
			label = "unknown"
		}
		r.cwdCache[cwd] = projectInfo{key: label, label: label}
		return label, label
	}

	dir := filepath.Clean(cwd)
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	if _, err := os.Stat(dir); err != nil {
		label := portableBase(cwd)
		if label == "" {
			label = "unknown"
		}
		r.cwdCache[cwd] = projectInfo{key: label, label: label}
		return label, label
	}

	root := r.findGitRoot(dir)
	if root != "" {
		label := filepath.Base(root)
		if label == "." || label == string(filepath.Separator) {
			label = root
		}
		r.cwdCache[cwd] = projectInfo{key: root, label: label}
		return root, label
	}

	label := filepath.Base(dir)
	if label == "." || label == string(filepath.Separator) {
		label = dir
	}
	if label == "" {
		label = "unknown"
	}
	r.cwdCache[cwd] = projectInfo{key: label, label: label}
	return label, label
}

func (r *projectResolver) findGitRoot(start string) string {
	start = filepath.Clean(start)
	if start == "" {
		return ""
	}

	visited := make([]string, 0, 16)
	dir := start
	for {
		if entry, ok := r.dirCache[dir]; ok {
			if entry.found {
				return entry.root
			}
			return ""
		}

		visited = append(visited, dir)
		if isGitRoot(dir) {
			for _, v := range visited {
				r.dirCache[v] = gitRootCacheEntry{root: dir, found: true}
			}
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	for _, v := range visited {
		r.dirCache[v] = gitRootCacheEntry{root: "", found: false}
	}
	return ""
}

func isGitRoot(dir string) bool {
	if dir == "" {
		return false
	}
	path := filepath.Join(dir, ".git")
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	// Worktrees can represent .git as a file pointing at the actual git dir.
	return info.IsDir() || (info.Mode()&os.ModeType) == 0
}

func looksLikeWindowsPath(value string) bool {
	// drive letter + colon (e.g. C:\foo) or UNC paths.
	if strings.HasPrefix(value, `\\`) {
		return true
	}
	if len(value) >= 2 && value[1] == ':' {
		return true
	}
	return strings.Contains(value, `\`)
}

func portableBase(value string) string {
	trimmed := strings.TrimRight(value, `/\`)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.ReplaceAll(trimmed, `\`, "/")
	parts := strings.Split(trimmed, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}
	return trimmed
}
