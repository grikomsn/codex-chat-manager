package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/grikomsn/codex-chat-manager/internal/session"
)

const (
	wideWidth  = 120
	wideHeight = 20
)

type mode int

const (
	modeListWide mode = iota
	modeListNarrow
	modePreview
	modeGroupDetail
	modeConfirm
)

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Select  key.Binding
	Filter  key.Binding
	System  key.Binding
	Status  key.Binding
	Enter   key.Binding
	Back    key.Binding
	Archive key.Binding
	Unarch  key.Binding
	Delete  key.Binding
	Resume  key.Binding
	Refresh key.Binding
	Help    key.Binding
	Quit    key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("k/up", "up")),
		Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("j/down", "down")),
		Select:  key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "toggle")),
		Filter:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		System:  key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "instructions")),
		Status:  key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "status")),
		Enter:   key.NewBinding(key.WithKeys("enter", "l", "right"), key.WithHelp("enter", "open")),
		Back:    key.NewBinding(key.WithKeys("esc", "h", "left"), key.WithHelp("esc", "back")),
		Archive: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "archive")),
		Unarch:  key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "unarchive")),
		Delete:  key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		Resume:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "resume")),
		Refresh: key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "refresh")),
		Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Select, k.Status, k.System, k.Archive, k.Unarch, k.Delete, k.Resume, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Enter, k.Back},
		{k.Select, k.Filter, k.Status, k.System},
		{k.Refresh, k.Archive, k.Unarch, k.Delete},
		{k.Resume, k.Help, k.Quit},
	}
}

type item struct {
	group session.SessionGroup
}

func (i item) Title() string { return i.group.Parent.DisplayTitle() }
func (i item) Description() string {
	return fmt.Sprintf("%s | %s | %s", i.group.Status, i.group.Parent.Subtitle(), i.group.AggregateAt.Format("2006-01-02 15:04"))
}
func (i item) FilterValue() string {
	return strings.Join([]string{i.group.Parent.ID, i.group.Parent.DisplayTitle(), i.group.Parent.CWD}, " ")
}

type confirmResultMsg struct {
	confirmed bool
}

type confirmDoneMsg struct{}

type model struct {
	cfg          session.Config
	store        *session.Store
	cache        *session.PreviewCache
	keys         keyMap
	help         help.Model
	list         list.Model
	childList    list.Model
	viewport     viewport.Model
	filterInput  textinput.Model
	mode         mode
	width        int
	height       int
	statusFilter string
	groups       []session.SessionGroup
	children     []session.SessionRecord
	selection    map[string]struct{}
	current      *session.SessionGroup
	currentDoc   session.PreviewDocument
	err          error
	confirmForm  *huh.Form
	confirmAct   session.ActionType
	confirmIDs   []string
	showSystem   bool
}

var (
	chromeStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#5f6b7a")).
			Padding(0, 1)
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#f7f2e8"))
	subtleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9aa4b2"))
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff5f5f"))
)

// Run starts the interactive session manager.
func Run(cfg session.Config, stdout, stderr io.Writer) error {
	m, err := initialModel(cfg)
	if err != nil {
		return err
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithOutput(stdout))
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(stderr, err)
		return err
	}
	return nil
}

func initialModel(cfg session.Config) (model, error) {
	store := session.NewStore(cfg)
	cache := session.NewPreviewCache()
	snapshot, err := store.LoadSnapshot()
	if err != nil {
		return model{}, err
	}
	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(2)
	delegate.ShowDescription = true
	l := list.New(nil, delegate, 0, 0)
	l.Title = "Codex Sessions"
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetShowStatusBar(false)
	l.DisableQuitKeybindings()

	cl := list.New(nil, delegate, 0, 0)
	cl.Title = "Grouped Children"
	cl.SetShowHelp(false)
	cl.SetShowPagination(false)
	cl.SetShowStatusBar(false)
	cl.DisableQuitKeybindings()

	ti := textinput.New()
	ti.Prompt = "Filter: "
	ti.CharLimit = 256

	vp := viewport.New(0, 0)
	keys := newKeyMap()
	m := model{
		cfg:          cfg,
		store:        store,
		cache:        cache,
		keys:         keys,
		help:         help.New(),
		list:         l,
		childList:    cl,
		viewport:     vp,
		filterInput:  ti,
		statusFilter: "all",
		groups:       snapshot.Groups,
		selection:    make(map[string]struct{}),
	}
	m.reloadList()
	return m, nil
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		m.syncPreview()
		return m, nil
	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}
		if key.Matches(msg, m.keys.Help) {
			m.help.ShowAll = !m.help.ShowAll
			return m, nil
		}
		if key.Matches(msg, m.keys.Refresh) {
			if err := m.refresh(); err != nil {
				m.err = err
			}
			return m, nil
		}
		if key.Matches(msg, m.keys.System) {
			m.showSystem = !m.showSystem
			m.syncPreview()
			return m, nil
		}
	}

	switch m.mode {
	case modeListWide, modeListNarrow:
		return m.updateList(msg)
	case modePreview:
		return m.updatePreview(msg)
	case modeGroupDetail:
		return m.updateGroupDetail(msg)
	case modeConfirm:
		return m.updateConfirm(msg)
	default:
		return m, nil
	}
}

func (m model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, m.keys.Status):
			m.nextStatusFilter()
			return m, nil
		case key.Matches(keyMsg, m.keys.Select):
			if group := m.selectedGroup(); group != nil {
				m.toggleGroup(group)
			}
			return m, nil
		case key.Matches(keyMsg, m.keys.Enter):
			if group := m.selectedGroup(); group != nil {
				if m.isWide() {
					if len(group.Children) > 0 {
						m.current = group
						m.loadChildren(group.Children)
						m.mode = modeGroupDetail
						return m, nil
					}
					m.current = group
					m.syncPreview()
					return m, nil
				}
				m.current = group
				m.syncPreview()
				m.mode = modePreview
				return m, nil
			}
		case key.Matches(keyMsg, m.keys.Archive):
			return m.beginConfirm(session.ActionArchive)
		case key.Matches(keyMsg, m.keys.Unarch):
			return m.beginConfirm(session.ActionUnarchive)
		case key.Matches(keyMsg, m.keys.Delete):
			return m.beginConfirm(session.ActionDelete)
		case key.Matches(keyMsg, m.keys.Resume):
			if group := m.selectedGroup(); group != nil {
				if err := m.store.Resume(nil, group.Parent.ID); err != nil {
					m.err = err
				}
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	if group := m.selectedGroup(); group != nil && m.isWide() {
		m.current = group
		m.syncPreview()
	}
	return m, cmd
}

func (m model) updatePreview(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && key.Matches(keyMsg, m.keys.Back) {
		m.mode = modeListNarrow
		return m, nil
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m model) updateGroupDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, m.keys.Back):
			if m.isWide() {
				m.mode = modeListWide
			} else {
				m.mode = modeListNarrow
			}
			return m, nil
		case key.Matches(keyMsg, m.keys.Select):
			if item, ok := m.childList.SelectedItem().(item); ok {
				m.toggleRecord(item.group.Parent.ID)
			}
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.childList, cmd = m.childList.Update(msg)
	if child, ok := m.childList.SelectedItem().(item); ok {
		g := child.group
		m.current = &g
		m.syncPreview()
	}
	return m, cmd
}

func (m model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case confirmDoneMsg:
		m.mode = modeListWide
		if !m.isWide() {
			m.mode = modeListNarrow
		}
		return m, nil
	case confirmResultMsg:
		if !msg.confirmed {
			if m.isWide() {
				m.mode = modeListWide
			} else {
				m.mode = modeListNarrow
			}
			return m, nil
		}
		if err := m.performConfirm(); err != nil {
			m.err = err
		}
		if m.isWide() {
			m.mode = modeListWide
		} else {
			m.mode = modeListNarrow
		}
		return m, nil
	}
	formModel := m.confirmForm
	newForm, cmd := formModel.Update(msg)
	typed, _ := newForm.(*huh.Form)
	m.confirmForm = typed
	if m.confirmForm.State == huh.StateCompleted {
		return m, func() tea.Msg { return confirmResultMsg{confirmed: m.confirmForm.GetBool("confirm")} }
	}
	if m.confirmForm.State == huh.StateAborted {
		return m, func() tea.Msg { return confirmDoneMsg{} }
	}
	return m, cmd
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}
	helpView := m.help.View(m.keys)
	statusLine := lipgloss.NewStyle().Foreground(lipgloss.Color("#a0a0a0")).Render(fmt.Sprintf("filter=%s selected=%d", m.statusFilter, len(m.selection)))
	errLine := ""
	if m.err != nil {
		errLine = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5f5f")).Render(m.err.Error()) + "\n"
	}
	switch m.mode {
	case modePreview:
		return errLine + m.viewport.View() + "\n" + statusLine + "\n" + helpView
	case modeGroupDetail:
		body := m.childList.View()
		if m.isWide() {
			body = lipgloss.JoinHorizontal(lipgloss.Top, m.renderChildPane(), m.renderPreviewPane())
		}
		return errLine + body + "\n" + statusLine + "\n" + helpView
	case modeConfirm:
		return errLine + m.confirmForm.View()
	default:
		if m.isWide() {
			return errLine + lipgloss.JoinHorizontal(lipgloss.Top, m.renderListPane(), m.renderPreviewPane()) + "\n" + statusLine + "\n" + helpView
		}
		return errLine + m.renderListPane() + "\n" + statusLine + "\n" + helpView
	}
}

func (m *model) resize() {
	footerHeight := 4
	bodyHeight := max(1, m.height-footerHeight)
	frameW := chromeStyle.GetHorizontalFrameSize()
	frameH := chromeStyle.GetVerticalFrameSize()
	if m.isWide() {
		m.mode = modeListWide
		left := max(30, m.width/3)
		right := max(30, m.width-left)
		m.list.SetSize(max(10, left-frameW), max(5, bodyHeight-frameH))
		m.childList.SetSize(max(10, left-frameW), max(5, bodyHeight-frameH))
		m.viewport.Width = max(10, right-frameW)
		m.viewport.Height = max(5, bodyHeight-frameH)
	} else {
		if m.mode == modeListWide {
			m.mode = modeListNarrow
		}
		m.list.SetSize(max(10, m.width-frameW), max(5, bodyHeight-frameH))
		m.childList.SetSize(max(10, m.width-frameW), max(5, bodyHeight-frameH))
		m.viewport.Width = max(10, m.width-frameW)
		m.viewport.Height = max(5, bodyHeight-frameH)
	}
	m.help.Width = m.width
}

func (m *model) reloadList() {
	items := make([]list.Item, 0, len(m.groups))
	for _, group := range m.filteredGroups() {
		items = append(items, item{group: group})
	}
	m.list.SetItems(items)
	if selected := m.selectedGroup(); selected != nil {
		m.current = selected
	} else if len(m.groups) > 0 {
		m.current = &m.groups[0]
	}
}

func (m *model) filteredGroups() []session.SessionGroup {
	return session.FilterGroups(m.groups, m.statusFilter, m.filterInput.Value(), false)
}

func (m *model) nextStatusFilter() {
	switch m.statusFilter {
	case "all":
		m.statusFilter = "active"
	case "active":
		m.statusFilter = "archived"
	default:
		m.statusFilter = "all"
	}
	m.reloadList()
}

func (m *model) selectedGroup() *session.SessionGroup {
	selected, ok := m.list.SelectedItem().(item)
	if !ok {
		return nil
	}
	group := selected.group
	return &group
}

func (m *model) loadChildren(children []session.SessionRecord) {
	items := make([]list.Item, 0, len(children))
	for _, child := range children {
		group := session.SessionGroup{Parent: child, Status: child.Status, AggregateAt: child.UpdatedAt, ParentExists: true}
		items = append(items, item{group: group})
	}
	m.childList.SetItems(items)
	if len(children) > 0 {
		group := session.SessionGroup{Parent: children[0], Status: children[0].Status, AggregateAt: children[0].UpdatedAt, ParentExists: true}
		m.current = &group
		m.syncPreview()
	}
}

func (m *model) syncPreview() {
	if m.current == nil {
		return
	}
	doc, err := m.cache.Load(m.current.Parent)
	if err != nil {
		m.err = err
		return
	}
	m.currentDoc = doc
	m.viewport.SetContent(session.RenderPreview(doc, m.viewport.Width, m.showSystem))
	m.viewport.GotoTop()
}

func (m *model) toggleGroup(group *session.SessionGroup) {
	for _, id := range group.CascadesTo {
		if _, ok := m.selection[id]; ok {
			delete(m.selection, id)
			continue
		}
		m.selection[id] = struct{}{}
	}
}

func (m *model) toggleRecord(id string) {
	if _, ok := m.selection[id]; ok {
		delete(m.selection, id)
		return
	}
	m.selection[id] = struct{}{}
}

func (m *model) selectedIDs() []string {
	ids := make([]string, 0, len(m.selection))
	for id := range m.selection {
		ids = append(ids, id)
	}
	return ids
}

func (m model) beginConfirm(action session.ActionType) (tea.Model, tea.Cmd) {
	ids := m.selectedIDs()
	if len(ids) == 0 {
		if group := m.selectedGroup(); group != nil {
			ids = append(ids, group.Parent.ID)
		}
	}
	if len(ids) == 0 {
		return m, nil
	}
	m.confirmAct = action
	m.confirmIDs = ids
	confirmed := false
	m.confirmForm = huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Key("confirm").
				Title(fmt.Sprintf("%s %d session(s)?", strings.Title(string(action)), len(ids))).
				Affirmative("Yes").
				Negative("No").
				Value(&confirmed),
		),
	)
	m.mode = modeConfirm
	return m, nil
}

func (m *model) performConfirm() error {
	var err error
	switch m.confirmAct {
	case session.ActionArchive:
		_, err = m.store.Archive(m.confirmIDs)
	case session.ActionUnarchive:
		_, err = m.store.Unarchive(m.confirmIDs)
	case session.ActionDelete:
		_, err = m.store.Delete(m.confirmIDs)
	}
	if err != nil {
		return err
	}
	m.selection = make(map[string]struct{})
	return m.refresh()
}

func (m *model) refresh() error {
	snapshot, err := m.store.LoadSnapshot()
	if err != nil {
		return err
	}
	m.groups = snapshot.Groups
	m.reloadList()
	m.syncPreview()
	return nil
}

func (m model) isWide() bool {
	return m.width >= wideWidth && m.height >= wideHeight
}

func (m model) renderListPane() string {
	title := "Sessions"
	if m.statusFilter != "all" {
		title += " [" + m.statusFilter + "]"
	}
	return chromeStyle.
		Width(m.list.Width()).
		Height(m.list.Height()).
		Render(titleStyle.Render(title) + "\n" + m.list.View())
}

func (m model) renderChildPane() string {
	return chromeStyle.
		Width(m.childList.Width()).
		Height(m.childList.Height()).
		Render(titleStyle.Render("Grouped Children") + "\n" + m.childList.View())
}

func (m model) renderPreviewPane() string {
	label := "Preview"
	if !m.showSystem {
		label += " | system hidden"
	} else {
		label += " | system shown"
	}
	return chromeStyle.
		Width(m.viewport.Width).
		Height(m.viewport.Height).
		Render(titleStyle.Render(label) + "\n" + m.viewport.View())
}
