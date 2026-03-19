package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProjectResolverUsesGitRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "subdir")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	resolver := newProjectResolver()
	key, label := resolver.Resolve(sub)
	if key != root {
		t.Fatalf("expected key %q, got %q", root, key)
	}
	if want := filepath.Base(root); label != want {
		t.Fatalf("expected label %q, got %q", want, label)
	}
}

func TestFilterGroupsOrdersByProjectStatusAndRecency(t *testing.T) {
	t.Parallel()
	t1 := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 3, 21, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)

	projectAKey := "/tmp/project-a"
	projectBKey := "/tmp/project-b"
	projectCKey := "/tmp/project-c"

	groups := []SessionGroup{
		{
			Parent:      SessionRecord{ID: "A-arch", Status: StatusArchived, ProjectKey: projectAKey, Project: "project-a"},
			Status:      StatusArchived,
			AggregateAt: t2,
		},
		{
			Parent:      SessionRecord{ID: "A-act", Status: StatusActive, ProjectKey: projectAKey, Project: "project-a"},
			Status:      StatusActive,
			AggregateAt: t1,
		},
		{
			Parent:      SessionRecord{ID: "B-mix", Status: StatusActive, ProjectKey: projectBKey, Project: "project-b"},
			Status:      StatusMixed,
			AggregateAt: t3,
		},
		{
			Parent:      SessionRecord{ID: "C-arch", Status: StatusArchived, ProjectKey: projectCKey, Project: "project-c"},
			Status:      StatusArchived,
			AggregateAt: t3,
		},
	}

	ordered := FilterGroups(groups, StatusFilterAll, "", false)
	if len(ordered) != len(groups) {
		t.Fatalf("expected %d groups, got %d", len(groups), len(ordered))
	}

	// Project A has best rank=active, newest=t2, so it should come before archived-only Project C
	// and before mixed Project B. Within Project A, active comes before archived even though it's older.
	want := []string{"A-act", "A-arch", "B-mix", "C-arch"}
	for i, id := range want {
		if ordered[i].Parent.ID != id {
			t.Fatalf("expected ordered[%d]=%q, got %q", i, id, ordered[i].Parent.ID)
		}
	}
}
