package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

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
	m.width = 70
	m.height = 30
	m.resize()
	if m.isWide() {
		t.Fatal("expected narrow mode")
	}
}

func TestDoubleTapArchiveArmsThenExecutesCascade(t *testing.T) {
	t.Parallel()
	cfg := makeTUIFixtureWithChildGroup(t)
	m, err := initialModel(cfg)
	if err != nil {
		t.Fatalf("initialModel() error = %v", err)
	}
	m.width = 100
	m.height = 30
	m.resize()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = updated.(model)

	if m.armedAct != session.ActionArchive {
		t.Fatalf("expected armed action archive, got %q", m.armedAct)
	}
	if got := m.keys.Archive.Help().Desc; !strings.Contains(got, "confirm archive (2)") {
		t.Fatalf("expected archive help to include confirm label, got %q", got)
	}

	snapshot, err := m.store.LoadSnapshot()
	if err != nil {
		t.Fatalf("LoadSnapshot() error = %v", err)
	}
	if got := snapshot.RecordsByID["11111111-1111-1111-1111-111111111111"].Status; got != session.StatusActive {
		t.Fatalf("expected parent to remain active after first tap, got %q", got)
	}
	if got := snapshot.RecordsByID["22222222-2222-2222-2222-222222222222"].Status; got != session.StatusActive {
		t.Fatalf("expected child to remain active after first tap, got %q", got)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = updated.(model)

	if m.armedAct != "" {
		t.Fatalf("expected double-tap to disarm after execution, got %q", m.armedAct)
	}
	if got := m.keys.Archive.Help().Key; got != "a a" {
		t.Fatalf("expected archive help to restore default key, got %q", got)
	}

	snapshot, err = m.store.LoadSnapshot()
	if err != nil {
		t.Fatalf("LoadSnapshot() error = %v", err)
	}
	if got := snapshot.RecordsByID["11111111-1111-1111-1111-111111111111"].Status; got != session.StatusArchived {
		t.Fatalf("expected parent to be archived after second tap, got %q", got)
	}
	if got := snapshot.RecordsByID["22222222-2222-2222-2222-222222222222"].Status; got != session.StatusArchived {
		t.Fatalf("expected child to be archived after second tap, got %q", got)
	}
}

func TestDoubleTapTimeoutClearsArming(t *testing.T) {
	t.Parallel()
	cfg := makeTUIFixture(t)
	m, err := initialModel(cfg)
	if err != nil {
		t.Fatalf("initialModel() error = %v", err)
	}
	m.width = 100
	m.height = 30
	m.resize()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = updated.(model)
	nonce := m.armedNonce

	updated, _ = m.Update(doubleTapExpiredMsg{nonce: nonce})
	m = updated.(model)

	if m.armedAct != "" {
		t.Fatalf("expected arming to clear on timeout, got %q", m.armedAct)
	}
	if got := m.keys.Archive.Help().Key; got != "a a" {
		t.Fatalf("expected archive help to restore default key, got %q", got)
	}

	snapshot, err := m.store.LoadSnapshot()
	if err != nil {
		t.Fatalf("LoadSnapshot() error = %v", err)
	}
	if got := snapshot.RecordsByID["11111111-1111-1111-1111-000000000001"].Status; got != session.StatusActive {
		t.Fatalf("expected session to remain active after timeout, got %q", got)
	}
}

func TestDoubleTapDeleteBlockedDoesNotArm(t *testing.T) {
	t.Parallel()
	cfg := makeTUIFixtureWithChildGroup(t)
	m, err := initialModel(cfg)
	if err != nil {
		t.Fatalf("initialModel() error = %v", err)
	}
	m.width = 100
	m.height = 30
	m.resize()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(model)

	if m.armedAct != "" {
		t.Fatalf("expected delete to remain unarmed when blocked, got %q", m.armedAct)
	}
	if got := m.keys.Delete.Help().Key; got != "d d" {
		t.Fatalf("expected delete help to keep default key, got %q", got)
	}
	if !strings.Contains(m.errorMsg, "delete blocked by active sessions") {
		t.Fatalf("expected delete blocked error, got %q", m.errorMsg)
	}
}

func TestDeleteClampsSelectionToNewLastItem(t *testing.T) {
	t.Parallel()
	cfg := makeTUIFixtureWithGroups(t, 3, false, false)
	m, err := initialModel(cfg)
	if err != nil {
		t.Fatalf("initialModel() error = %v", err)
	}
	m.width = 100
	m.height = 30
	m.resize()

	ids := []string{
		"11111111-1111-1111-1111-000000000001",
		"11111111-1111-1111-1111-000000000002",
		"11111111-1111-1111-1111-000000000003",
	}
	if _, err := m.store.Archive(ids); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	if err := m.refresh(); err != nil {
		t.Fatalf("refresh() error = %v", err)
	}

	m.statusFilter = session.StatusFilterArchived
	m.reloadList()

	items := m.list.VisibleItems()
	if len(items) != 3 {
		t.Fatalf("expected 3 archived items, got %d", len(items))
	}
	m.list.Select(2)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(model)

	items = m.list.VisibleItems()
	if len(items) != 2 {
		t.Fatalf("expected 2 items after delete, got %d", len(items))
	}
	if got := m.list.Index(); got != 1 {
		t.Fatalf("expected selection to clamp to new last index=1, got %d", got)
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
	if strings.Contains(ansi.Strip(m.viewport.View()), "developer system instructions") {
		t.Fatalf("expected hidden system instructions, got %q", m.viewport.View())
	}
	m.showSystem = true
	m.syncPreview()
	if !strings.Contains(ansi.Strip(m.viewport.View()), "developer system instructions") {
		t.Fatalf("expected visible system instructions, got %q", m.viewport.View())
	}
}

func TestResizePreservesPreviewScroll(t *testing.T) {
	t.Parallel()
	cfg := makeTUIFixtureWithGroups(t, 1, false, true)
	m, err := initialModel(cfg)
	if err != nil {
		t.Fatalf("initialModel() error = %v", err)
	}
	m.width = 90
	m.height = 18
	m.resize()
	group := m.groups[0]
	m.current = &group
	m.mode = modePreview
	m.syncPreview()
	m.viewport.SetContent(strings.Repeat("preview line\n", 100))
	m.viewport.SetYOffset(6)

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m = updated.(model)

	if m.viewport.YOffset == 0 {
		t.Fatal("expected preview offset to survive resize")
	}
}

func TestMouseWheelRoutesByPaneInWideMode(t *testing.T) {
	t.Parallel()
	cfg := makeTUIFixtureWithGroups(t, 6, false, true)
	m, err := initialModel(cfg)
	if err != nil {
		t.Fatalf("initialModel() error = %v", err)
	}
	m.width = 140
	m.height = 20
	m.resize()
	group := m.groups[0]
	m.current = &group
	m.syncPreview()
	m.viewport.SetContent(strings.Repeat("preview line\n", 100))
	layout := m.layout()
	currentID := m.current.Parent.ID

	updated, _ := m.Update(tea.MouseMsg{
		X:      layout.previewPane.x + 1,
		Y:      layout.previewPane.y + 1,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
	})
	m = updated.(model)
	if m.viewport.YOffset == 0 {
		t.Fatal("expected preview wheel scroll in preview pane")
	}
	if m.list.Index() != 0 {
		t.Fatalf("expected preview scroll to leave list index unchanged, got %d", m.list.Index())
	}

	updated, _ = m.Update(tea.MouseMsg{
		X:      layout.listPane.x + 1,
		Y:      layout.listPane.y + 1,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonWheelDown,
	})
	m = updated.(model)
	if m.listScroll == 0 {
		t.Fatal("expected list pane wheel to scroll visible sessions")
	}
	if m.list.Index() != 0 {
		t.Fatalf("expected list pane wheel to keep active index unchanged, got %d", m.list.Index())
	}
	if m.current == nil || m.current.Parent.ID != currentID {
		t.Fatal("expected list pane wheel to keep active session unchanged")
	}
}

func TestMouseClickOpensPreviewInNarrowMode(t *testing.T) {
	t.Parallel()
	cfg := makeTUIFixtureWithGroups(t, 4, false, false)
	m, err := initialModel(cfg)
	if err != nil {
		t.Fatalf("initialModel() error = %v", err)
	}
	m.width = 70
	m.height = 18
	m.resize()
	layout := m.layout()
	content := layout.listPane.contentRect(chromeStyle)
	rowY := content.y + paneHeaderHeight + listDelegateHeight + listDelegateSpacing
	expected := m.list.VisibleItems()[1].(item).group.Parent.DisplayTitle()

	updated, _ := m.Update(tea.MouseMsg{
		X:      content.x + 1,
		Y:      rowY,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	m = updated.(model)

	if m.mode != modePreview {
		t.Fatalf("expected narrow click to open preview, got mode %v", m.mode)
	}
	group := m.selectedGroup()
	if group == nil || group.Parent.DisplayTitle() != expected {
		t.Fatalf("expected second item selected, got %#v", group)
	}
}

func TestMouseClickScrollbarScrollsListWithoutOpeningPreview(t *testing.T) {
	t.Parallel()
	cfg := makeTUIFixtureWithGroups(t, 24, false, false)
	m, err := initialModel(cfg)
	if err != nil {
		t.Fatalf("initialModel() error = %v", err)
	}
	m.width = 70
	m.height = 14
	m.resize()
	if m.mode != modeListNarrow {
		t.Fatalf("expected narrow list mode, got %v", m.mode)
	}
	layout := m.layout()
	content := layout.listPane.contentRect(chromeStyle)
	sbX := content.x + (m.list.Width() - 1)
	sbY := content.y + paneHeaderHeight

	updated, _ := m.Update(tea.MouseMsg{
		X:      sbX,
		Y:      sbY + m.list.Height() - 1,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	m = updated.(model)

	if m.mode != modeListNarrow {
		t.Fatalf("expected scrollbar click to keep list mode, got %v", m.mode)
	}
	if m.listScroll == 0 {
		t.Fatal("expected scrollbar click to scroll the list")
	}
}

func TestMouseClickScrollbarScrollsPreviewInWideMode(t *testing.T) {
	t.Parallel()
	cfg := makeTUIFixtureWithGroups(t, 1, false, true)
	m, err := initialModel(cfg)
	if err != nil {
		t.Fatalf("initialModel() error = %v", err)
	}
	m.width = 140
	m.height = 20
	m.resize()
	group := m.groups[0]
	m.current = &group
	m.syncPreview()
	layout := m.layout()
	content := layout.previewPane.contentRect(chromeStyle)
	sbX := content.x + (m.viewport.Width - 1)
	sbY := content.y + paneHeaderHeight

	updated, _ := m.Update(tea.MouseMsg{
		X:      sbX,
		Y:      sbY + m.viewport.Height - 1,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	m = updated.(model)

	if m.viewport.YOffset == 0 {
		t.Fatal("expected preview scrollbar click to scroll the viewport")
	}
}

func TestMouseDragScrollbarScrollsPreview(t *testing.T) {
	t.Parallel()
	cfg := makeTUIFixtureWithGroups(t, 1, false, true)
	m, err := initialModel(cfg)
	if err != nil {
		t.Fatalf("initialModel() error = %v", err)
	}
	m.width = 140
	m.height = 18
	m.resize()
	group := m.groups[0]
	m.current = &group
	m.syncPreview()

	layout := m.layout()
	content := layout.previewPane.contentRect(chromeStyle)
	sbX := content.x + (m.viewport.Width - 1)
	sbY := content.y + paneHeaderHeight

	updated, _ := m.Update(tea.MouseMsg{
		X:      sbX,
		Y:      sbY,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	m = updated.(model)
	if m.dragTarget != scrollDragPreview {
		t.Fatalf("expected preview drag target, got %v", m.dragTarget)
	}
	start := m.viewport.YOffset

	updated, _ = m.Update(tea.MouseMsg{
		X:      sbX,
		Y:      sbY + m.viewport.Height - 1,
		Action: tea.MouseActionMotion,
		Button: tea.MouseButtonLeft,
	})
	m = updated.(model)
	if m.viewport.YOffset <= start {
		t.Fatalf("expected drag motion to increase offset, got %d <= %d", m.viewport.YOffset, start)
	}

	updated, _ = m.Update(tea.MouseMsg{
		X:      sbX,
		Y:      sbY + m.viewport.Height - 1,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonNone,
	})
	m = updated.(model)
	if m.dragTarget != scrollDragNone {
		t.Fatalf("expected drag to stop on release, got %v", m.dragTarget)
	}
}

func TestViewFitsTerminalHeightOnLaunch(t *testing.T) {
	t.Parallel()
	cfg := makeTUIFixtureWithGroups(t, 6, false, true)
	m, err := initialModel(cfg)
	if err != nil {
		t.Fatalf("initialModel() error = %v", err)
	}
	m.width = 80
	m.height = 24
	m.resize()
	group := m.groups[0]
	m.current = &group
	m.syncPreview()

	if got := lipgloss.Height(m.View()); got > m.height {
		t.Fatalf("expected view height <= terminal height, got view=%d term=%d", got, m.height)
	}
}

func makeTUIFixture(t *testing.T) session.Config {
	t.Helper()
	return makeTUIFixtureWithGroups(t, 1, false, false)
}

func makeTUIFixtureWithContext(t *testing.T) session.Config {
	t.Helper()
	return makeTUIFixtureWithGroups(t, 1, true, false)
}

func makeTUIFixtureWithChildGroup(t *testing.T) session.Config {
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
	parentID := "11111111-1111-1111-1111-111111111111"
	childID := "22222222-2222-2222-2222-222222222222"

	parentBody := `{"type":"session_meta","payload":{"id":"` + parentID + `","cwd":"/tmp/app","source":"vscode"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"user_message","message":"parent title"}}` + "\n"
	childBody := `{"type":"session_meta","payload":{"id":"` + childID + `","cwd":"/tmp/app","source":{"subagent":{"thread_spawn":{"parent_thread_id":"` + parentID + `","agent_nickname":"Faraday","agent_role":"explorer"}}}}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"user_message","message":"child title"}}` + "\n"

	parentName := "rollout-2026-03-19T10-42-01-" + parentID + ".jsonl"
	childName := "rollout-2026-03-19T10-42-02-" + childID + ".jsonl"
	if err := os.WriteFile(filepath.Join(path, parentName), []byte(parentBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, childName), []byte(childBody), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func makeTUIFixtureWithGroups(t *testing.T, count int, withContext, longPreview bool) session.Config {
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
	for i := 1; i <= count; i++ {
		id := fmt.Sprintf("11111111-1111-1111-1111-%012d", i)
		body := fmt.Sprintf(`{"type":"session_meta","payload":{"id":"%s","cwd":"/tmp/app%d","source":"vscode"}}`+"\n", id, i)
		if withContext && i == 1 {
			body += `{"type":"response_item","payload":{"type":"message","role":"developer","content":[{"type":"input_text","text":"developer system instructions"}]}}` + "\n"
		}
		body += fmt.Sprintf(`{"type":"event_msg","payload":{"type":"user_message","message":"test title %02d"}}`+"\n", i)
		if longPreview && i == 1 {
			for line := 0; line < 40; line++ {
				body += fmt.Sprintf(`{"type":"event_msg","payload":{"type":"user_message","message":"preview line %02d"}}`+"\n", line)
			}
		}
		name := fmt.Sprintf("rollout-2026-03-19T10-42-%02d-%s.jsonl", i, id)
		if err := os.WriteFile(filepath.Join(path, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return cfg
}
