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
	"github.com/charmbracelet/x/ansi"
	"github.com/grikomsn/codex-chat-manager/internal/session"
)

const (
	minWideListWidth    = 36
	minWidePreviewWidth = 44
	minWideHeight       = 12
	paneHeaderHeight    = 1
	listDelegateHeight  = 2
	listDelegateSpacing = 1
	mouseWheelStep      = 3
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

type box struct {
	x      int
	y      int
	width  int
	height int
}

func (b box) contains(x, y int) bool {
	return x >= b.x && x < b.x+b.width && y >= b.y && y < b.y+b.height
}

func (b box) contentRect(style lipgloss.Style) box {
	left := style.GetBorderLeftSize() + style.GetPaddingLeft()
	right := style.GetBorderRightSize() + style.GetPaddingRight()
	top := style.GetBorderTopSize() + style.GetPaddingTop()
	bottom := style.GetBorderBottomSize() + style.GetPaddingBottom()
	return box{
		x:      b.x + left,
		y:      b.y + top,
		width:  max(0, b.width-left-right),
		height: max(0, b.height-top-bottom),
	}
}

type viewLayout struct {
	body        box
	listPane    box
	childPane   box
	previewPane box
}

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
	listScroll   int
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
	sized        bool
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
	itemStyles = list.NewDefaultDelegate().Styles
)

// Run starts the interactive session manager.
func Run(cfg session.Config, stdout, stderr io.Writer) error {
	m, err := initialModel(cfg)
	if err != nil {
		return err
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithOutput(stdout))
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
	delegate.SetHeight(listDelegateHeight)
	delegate.SetSpacing(listDelegateSpacing)
	delegate.ShowDescription = true
	l := list.New(nil, delegate, 0, 0)
	l.Title = "Codex Sessions"
	l.SetShowTitle(false)
	l.SetShowFilter(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetShowStatusBar(false)
	l.DisableQuitKeybindings()

	cl := list.New(nil, delegate, 0, 0)
	cl.Title = "Grouped Children"
	cl.SetShowTitle(false)
	cl.SetShowFilter(false)
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
		firstSize := !m.sized
		m.sized = true
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		m.syncPreviewPreserveOffset()
		if firstSize {
			return m, tea.ClearScreen
		}
		return m, nil
	case tea.MouseMsg:
		if cmd, handled := m.handleMouse(msg); handled {
			return m, cmd
		}
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
			m.syncPreviewPreserveOffset()
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
	m.ensureListSelectionVisible()
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
	statusLine := m.renderStatusLine()
	errLine := m.renderErrorLine()
	parts := make([]string, 0, 4)
	if errLine != "" {
		parts = append(parts, errLine)
	}
	switch m.mode {
	case modePreview:
		parts = append(parts, m.renderPreviewPane())
	case modeGroupDetail:
		body := m.renderPreviewPane()
		if m.isWide() {
			body = lipgloss.JoinHorizontal(lipgloss.Top, m.renderChildPane(), m.renderPreviewPane())
		}
		parts = append(parts, body)
	case modeConfirm:
		parts = append(parts, m.confirmForm.View())
		return strings.Join(parts, "\n")
	default:
		if m.isWide() {
			parts = append(parts, lipgloss.JoinHorizontal(lipgloss.Top, m.renderListPane(), m.renderPreviewPane()))
		} else {
			parts = append(parts, m.renderListPane())
		}
	}
	parts = append(parts, statusLine, helpView)
	return strings.Join(parts, "\n")
}

func (m *model) resize() {
	m.help.Width = max(0, m.width)
	m.syncModeToSize()
	layout := m.layout()
	m.list.SetSize(m.componentWidth(layout.listPane), m.componentHeight(layout.listPane))
	m.childList.SetSize(m.componentWidth(layout.childPane), m.componentHeight(layout.childPane))
	m.viewport.Width = m.componentWidth(layout.previewPane)
	m.viewport.Height = m.componentHeight(layout.previewPane)
	m.clampListScroll()
}

func (m *model) reloadList() {
	filtered := m.filteredGroups()
	items := make([]list.Item, 0, len(filtered))
	for _, group := range filtered {
		items = append(items, item{group: group})
	}
	m.list.SetItems(items)
	if len(items) > 0 && m.list.SelectedItem() == nil {
		m.list.Select(0)
	}
	m.clampListScroll()
	if selected := m.selectedGroup(); selected != nil {
		m.current = selected
	} else if len(filtered) > 0 {
		m.current = &filtered[0]
	} else {
		m.current = nil
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
	m.syncPreviewWithReset(true)
}

func (m *model) syncPreviewPreserveOffset() {
	m.syncPreviewWithReset(false)
}

func (m *model) syncPreviewWithReset(reset bool) {
	if m.current == nil {
		return
	}
	offset := m.viewport.YOffset
	doc, err := m.cache.Load(m.current.Parent)
	if err != nil {
		m.err = err
		return
	}
	m.currentDoc = doc
	m.viewport.SetContent(session.RenderPreview(doc, m.viewport.Width, m.showSystem))
	if reset {
		m.viewport.GotoTop()
		return
	}
	m.viewport.SetYOffset(offset)
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
	return m.width >= minWideListWidth+minWidePreviewWidth && m.height >= minWideHeight
}

func (m model) renderListPane() string {
	title := "Sessions"
	if m.statusFilter != "all" {
		title += " [" + m.statusFilter + "]"
	}
	return m.renderPane(title, m.renderScrollableList(m.list.VisibleItems(), m.list.Width(), m.list.Height(), m.list.Index(), m.listScroll), m.list.Width(), m.list.Height())
}

func (m model) renderChildPane() string {
	return m.renderPane("Grouped Children", m.childList.View(), m.childList.Width(), m.childList.Height())
}

func (m model) renderPreviewPane() string {
	label := "Preview"
	if !m.showSystem {
		label += " | system hidden"
	} else {
		label += " | system shown"
	}
	return m.renderPane(label, m.viewport.View(), m.viewport.Width, m.viewport.Height)
}

func (m model) renderPane(title, body string, width, height int) string {
	renderWidth := width + chromeStyle.GetPaddingLeft() + chromeStyle.GetPaddingRight()
	renderHeight := paneHeaderHeight + height + chromeStyle.GetPaddingTop() + chromeStyle.GetPaddingBottom()
	return chromeStyle.
		Width(renderWidth).
		Height(renderHeight).
		Render(titleStyle.Render(title) + "\n" + body)
}

func (m model) renderStatusLine() string {
	return subtleStyle.Width(m.width).Render(fmt.Sprintf("filter=%s selected=%d", m.statusFilter, len(m.selection)))
}

func (m model) renderErrorLine() string {
	if m.err == nil {
		return ""
	}
	return errorStyle.Width(m.width).Render(m.err.Error())
}

func (m *model) syncModeToSize() {
	if m.mode == modeConfirm {
		return
	}
	if m.isWide() {
		if m.mode == modeListNarrow || m.mode == modePreview {
			m.mode = modeListWide
		}
		return
	}
	switch m.mode {
	case modeListWide:
		m.mode = modeListNarrow
	case modeGroupDetail:
		if m.current != nil {
			m.mode = modePreview
		} else {
			m.mode = modeListNarrow
		}
	}
}

func (m model) layout() viewLayout {
	errorHeight := renderHeight(m.renderErrorLine())
	statusHeight := renderHeight(m.renderStatusLine())
	helpHeight := renderHeight(m.help.View(m.keys))
	bodyHeight := max(1, m.height-errorHeight-statusHeight-helpHeight)
	layout := viewLayout{
		body: box{x: 0, y: errorHeight, width: m.width, height: bodyHeight},
	}
	if m.isWide() {
		leftWidth := clamp(m.width/3, minWideListWidth, m.width-minWidePreviewWidth)
		layout.listPane = box{x: 0, y: errorHeight, width: leftWidth, height: bodyHeight}
		layout.childPane = layout.listPane
		layout.previewPane = box{x: leftWidth, y: errorHeight, width: m.width - leftWidth, height: bodyHeight}
		return layout
	}
	layout.listPane = layout.body
	layout.childPane = layout.body
	layout.previewPane = layout.body
	return layout
}

func (m model) componentWidth(pane box) int {
	return max(1, pane.width-chromeStyle.GetHorizontalFrameSize())
}

func (m model) componentHeight(pane box) int {
	return max(1, pane.height-chromeStyle.GetVerticalFrameSize()-paneHeaderHeight)
}

func (m *model) handleMouse(msg tea.MouseMsg) (tea.Cmd, bool) {
	if m.mode == modeConfirm || m.width == 0 || m.height == 0 {
		return nil, false
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown:
		return m.handleMouseWheel(msg), true
	case tea.MouseButtonLeft:
		if msg.Action != tea.MouseActionPress {
			return nil, true
		}
		return m.handleMouseClick(msg), true
	default:
		return nil, true
	}
}

func (m *model) handleMouseWheel(msg tea.MouseMsg) tea.Cmd {
	layout := m.layout()
	switch m.mode {
	case modePreview:
		if layout.previewPane.contains(msg.X, msg.Y) {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return cmd
		}
	case modeGroupDetail:
		if m.isWide() && layout.previewPane.contains(msg.X, msg.Y) {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return cmd
		}
		if layout.childPane.contains(msg.X, msg.Y) {
			m.scrollChildList(msg)
		}
	default:
		if m.isWide() && layout.previewPane.contains(msg.X, msg.Y) {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return cmd
		}
		if layout.listPane.contains(msg.X, msg.Y) {
			m.scrollList(msg)
		}
	}
	return nil
}

func (m *model) handleMouseClick(msg tea.MouseMsg) tea.Cmd {
	layout := m.layout()
	switch m.mode {
	case modePreview:
		return nil
	case modeGroupDetail:
		if !layout.childPane.contains(msg.X, msg.Y) {
			return nil
		}
		index := listIndexAtPosition(m.childList, layout.childPane, msg, m.childList.Paginator.Page*m.childList.Paginator.PerPage)
		if index < 0 {
			return nil
		}
		m.childList.Select(index)
		if child, ok := m.childList.SelectedItem().(item); ok {
			group := child.group
			m.current = &group
			m.syncPreview()
		}
	default:
		if !layout.listPane.contains(msg.X, msg.Y) {
			return nil
		}
		index := listIndexAtPosition(m.list, layout.listPane, msg, m.listScroll)
		if index < 0 {
			return nil
		}
		wasSelected := index == m.list.Index()
		m.list.Select(index)
		group := m.selectedGroup()
		if group == nil {
			return nil
		}
		m.current = group
		if !m.isWide() {
			m.syncPreview()
			m.mode = modePreview
			return nil
		}
		if wasSelected && len(group.Children) > 0 {
			m.current = group
			m.loadChildren(group.Children)
			m.mode = modeGroupDetail
			return nil
		}
		m.syncPreview()
	}
	return nil
}

func (m *model) scrollList(msg tea.MouseMsg) {
	delta := mouseWheelStep
	if msg.Button == tea.MouseButtonWheelUp {
		delta = -delta
	}
	m.listScroll = clamp(m.listScroll+delta, 0, m.maxListScroll())
}

func (m *model) scrollChildList(msg tea.MouseMsg) {
	scrollListModel(&m.childList, msg)
	if child, ok := m.childList.SelectedItem().(item); ok {
		group := child.group
		m.current = &group
		m.syncPreview()
	}
}

func scrollListModel(l *list.Model, msg tea.MouseMsg) {
	items := l.VisibleItems()
	if len(items) == 0 || l.FilterState() == list.Filtering {
		return
	}
	delta := mouseWheelStep
	if msg.Button == tea.MouseButtonWheelUp {
		delta = -delta
	}
	index := clamp(l.Index()+delta, 0, len(items)-1)
	l.Select(index)
}

func listIndexAtPosition(l list.Model, pane box, msg tea.MouseMsg, start int) int {
	content := pane.contentRect(chromeStyle)
	row := msg.Y - content.y - paneHeaderHeight
	if row < 0 || row >= l.Height() {
		return -1
	}
	indexOnPage := row / (listDelegateHeight + listDelegateSpacing)
	items := l.VisibleItems()
	index := start + indexOnPage
	if index < start || index >= len(items) {
		return -1
	}
	return index
}

func clamp(value, lower, upper int) int {
	if value < lower {
		return lower
	}
	if value > upper {
		return upper
	}
	return value
}

func renderHeight(s string) int {
	if s == "" {
		return 0
	}
	return lipgloss.Height(s)
}

func (m *model) clampListScroll() {
	m.listScroll = clamp(m.listScroll, 0, m.maxListScroll())
}

func (m model) maxListScroll() int {
	return max(0, len(m.list.VisibleItems())-m.visibleListItemCount())
}

func (m model) visibleListItemCount() int {
	if m.list.Height() <= 0 {
		return 0
	}
	return max(1, (m.list.Height()+listDelegateSpacing)/(listDelegateHeight+listDelegateSpacing))
}

func (m *model) ensureListSelectionVisible() {
	items := m.list.VisibleItems()
	if len(items) == 0 {
		m.listScroll = 0
		return
	}
	index := clamp(m.list.Index(), 0, len(items)-1)
	visible := m.visibleListItemCount()
	if visible <= 0 {
		m.listScroll = 0
		return
	}
	if index < m.listScroll {
		m.listScroll = index
	} else if index >= m.listScroll+visible {
		m.listScroll = index - visible + 1
	}
	m.clampListScroll()
}

func (m model) renderScrollableList(items []list.Item, width, height, selectedIndex, scroll int) string {
	if height <= 0 {
		return ""
	}
	lines := make([]string, 0, height)
	if len(items) == 0 {
		lines = append(lines, subtleStyle.Render("No sessions found."))
		for len(lines) < height {
			lines = append(lines, "")
		}
		return strings.Join(lines[:height], "\n")
	}
	scroll = clamp(scroll, 0, max(0, len(items)-1))
	for index := scroll; index < len(items) && len(lines) < height; index++ {
		listItem, ok := items[index].(item)
		if !ok {
			continue
		}
		titleLine, descLine := renderSessionItem(listItem, width, index == selectedIndex)
		lines = append(lines, titleLine)
		if len(lines) >= height {
			break
		}
		lines = append(lines, descLine)
		if len(lines) >= height {
			break
		}
		lines = append(lines, "")
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines[:height], "\n")
}

func renderSessionItem(listItem item, width int, selected bool) (string, string) {
	textWidth := max(1, width-itemStyles.NormalTitle.GetPaddingLeft()-itemStyles.NormalTitle.GetPaddingRight())
	title := ansi.Truncate(listItem.Title(), textWidth, "…")
	description := listItem.Description()
	if line := strings.Split(description, "\n"); len(line) > 0 {
		description = line[0]
	}
	description = ansi.Truncate(description, textWidth, "…")
	if selected {
		return itemStyles.SelectedTitle.Render(title), itemStyles.SelectedDesc.Render(description)
	}
	return itemStyles.NormalTitle.Render(title), itemStyles.NormalDesc.Render(description)
}
