// Package ui is the Bubble Tea interface of azt.
package ui

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/layout"
	"github.com/ingvarch/azt/internal/ui/data"
	"github.com/ingvarch/azt/internal/ui/sidebar"
	"github.com/ingvarch/azt/internal/ui/views"
)

const (
	minPaneWidth  = 12
	minPaneHeight = 4
	resizeStep    = 0.05
	sidebarWidth  = 32

	sidebarHints       = "j/k move  l expand  h collapse  r refresh  enter open  ^w l panes  ^w e hide  : command  ? help  q quit"
	sidebarPrefixHints = "l/→ panes  e hide  t tab  n/p/1-9 switch  , rename  ? help  esc cancel"
	normalHints        = "enter open  esc back  / filter  : command  r refresh  ? help  ^w v/s split  ^w hjkl/←↓↑→ focus  ^w t tab  q quit"
	pathSep            = " › "
	pathCut            = "… › "
	prefixHints        = "v/s split  hjkl/←↓↑→ focus  q close  z zoom  = equal  <>+- resize  t tab  n/p/1-9 switch  e explorer  , rename  ? help  esc cancel"
	filterHints        = "enter apply  esc clear"
	renameHints        = "enter save  esc cancel"
	paletteHints       = "↑↓ choose  tab complete  enter run  esc close"
	renamePrompt       = "rename tab: "
)

type mode int

const (
	modeNormal mode = iota
	modePrefix
	modeFilter
	modePalette
	modeRename
	modeHelp
)

var (
	// arrows after the prefix work like h, j, k, l.
	arrows = map[string]string{"left": "h", "down": "j", "up": "k", "right": "l"}

	sidebarPrefixKeys = []string{"l", "e", "t", ",", "?", "n", "p", "1", "2", "3", "4", "5", "6", "7", "8", "9"}

	focusColor   = lipgloss.Color("12")
	borderColor  = lipgloss.Color("8")
	reverseStyle = lipgloss.NewStyle().Reverse(true)
	hintStyle    = lipgloss.NewStyle().Faint(true)
	boldStyle    = lipgloss.NewStyle().Bold(true)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Red)
	border       = lipgloss.NormalBorder()
)

type tab struct {
	root   *layout.Node
	focus  layout.PaneID
	zoomed bool
	name   string // custom label; empty for the title of the focused view
}

// Model is the root model: tabs of split panes, each pane holding a stack of
// views (the last one is shown). All views share one data cache.
type Model struct {
	newView       func() views.View
	cache         *data.Cache
	tabs          []tab
	active        int
	stacks        map[layout.PaneID][]views.View
	nextID        layout.PaneID
	width, height int
	mode          mode
	input         textinput.Model // serves the filter and the rename
	palette       paletteModel    // the command dialog, while the mode is palette
	notice        string          // command error shown until the next key
	now           func() time.Time
	drag          *drag
	last          *lastPress

	sidebar        sidebar.Model
	sidebarVisible bool
	sidebarFocused bool
}

// New returns a model with one tab and one pane; newView makes each new view.
func New(newView func() views.View, fetch data.Fetcher) Model {
	m := newModel(newView, fetch)
	m.tabs = []tab{{root: layout.Leaf(0), focus: 0}}
	m.stacks = map[layout.PaneID][]views.View{0: {newView()}}
	m.nextID = 1
	return m
}

// newModel returns a model without tabs.
func newModel(newView func() views.View, fetch data.Fetcher) Model {
	input := textinput.New()
	input.Prompt = ""
	styles := textinput.DefaultStyles(true)
	styles.Cursor.Blink = false
	input.SetStyles(styles)
	// Terminal paste arrives as tea.PasteMsg; ctrl+v would read the clipboard
	// into a message that never reaches the input.
	input.KeyMap.Paste.SetEnabled(false)
	return Model{
		input:   input,
		now:     time.Now,
		newView: newView,
		cache:   data.NewCache(fetch),
		stacks:  map[layout.PaneID][]views.View{},

		sidebar:        sidebar.New(),
		sidebarVisible: true,
	}
}

// Init starts every view of every stack of every tab, and the sidebar.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.sidebar.Init()}
	for _, t := range m.tabs {
		for _, id := range t.root.Panes() {
			for _, v := range m.stacks[id] {
				cmds = append(cmds, v.Init())
			}
		}
	}
	return tea.Batch(cmds...)
}

// top returns the view shown in pane id.
func (m Model) top(id layout.PaneID) views.View {
	s := m.stacks[id]
	return s[len(s)-1]
}

// Update routes keys, mouse and data messages to the prefix, sidebar or views.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.key(msg)
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.focusSidebar(m.sidebarFocused && m.sidebarDrawn())
		m.resizeViews()
		return m, nil
	case data.FetchMsg:
		return m, m.cache.Request(msg)
	case data.ResultMsg:
		m.cache.Store(msg)
	case tea.MouseMsg:
		return m.mouse(msg)
	case tea.PasteMsg:
		if m.mode == modePalette {
			m.palette = m.palette.paste(msg.Content)
			return m, nil
		}
		if m.typing() {
			cmd := m.editInput(msg)
			return m, cmd
		}
	}
	cmd := m.broadcast(msg)
	return m, cmd
}

func (m Model) key(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" {
		return m, tea.Quit
	}
	m.notice = ""
	m.drag, m.last = nil, nil
	switch {
	case m.mode == modePrefix:
		m.mode = modeNormal
		return m.runPrefix(k)
	case m.mode == modeHelp:
		m.mode = modeNormal
		return m, nil
	case m.mode == modePalette:
		return m.paletteKey(msg)
	case m.typing():
		return m.inputKey(msg)
	}
	if m.openOverlay(k) {
		return m, nil
	}
	if m.sidebarFocused && k != "q" && k != "ctrl+w" {
		return m.sidebarKey(msg)
	}
	switch k {
	case "q":
		return m, tea.Quit
	case "ctrl+w":
		m.mode = modePrefix
		return m, nil
	case "enter":
		cmd := m.open(m.tabs[m.active].focus)
		return m, cmd
	case "esc":
		m.back(m.tabs[m.active].focus)
		return m, nil
	}
	cmd := m.sendTop(m.tabs[m.active].focus, msg)
	return m, cmd
}

// openOverlay opens the filter, the command dialog or help for k; false when
// k opens none of them.
func (m *Model) openOverlay(k string) bool {
	switch k {
	case "/":
		m.startFilter()
	case ":":
		m.mode, m.palette = modePalette, newPalette()
	case "?":
		m.mode = modeHelp
	default:
		return false
	}
	return true
}

// startInput enters input mode md with the input set to text, cursor at the end.
func (m *Model) startInput(md mode, text string) {
	m.mode = md
	m.input.SetValue(text)
	m.input.CursorEnd()
	m.input.Focus()
}

// startFilter enters filter mode if the focused pane shows a filterable view.
func (m *Model) startFilter() {
	if m.sidebarFocused {
		return
	}
	if f, ok := m.top(m.tabs[m.active].focus).(views.Filterer); ok {
		m.startInput(modeFilter, f.Filter())
	}
}

// typing reports whether keys and pastes go to the input.
func (m Model) typing() bool {
	return m.mode == modeFilter || m.mode == modeRename
}

// inputKey handles a key in filter and rename mode.
func (m Model) inputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.finishInput()
		return m, nil
	case "esc":
		if m.mode == modeFilter {
			m.setFilter(m.tabs[m.active].focus, "")
		}
		m.mode = modeNormal
		return m, nil
	}
	cmd := m.editInput(msg)
	return m, cmd
}

// editInput passes msg to the input; in filter mode the focused view follows it.
func (m *Model) editInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.mode == modeFilter {
		m.setFilter(m.tabs[m.active].focus, m.input.Value())
	}
	return cmd
}

// finishInput applies the input on enter and returns to normal mode.
func (m *Model) finishInput() {
	if m.mode == modeRename {
		m.tabs[m.active].name = strings.TrimSpace(m.input.Value())
	}
	m.mode = modeNormal
}

// runCommand runs a palette command; an error becomes the notice.
func (m *Model) runCommand(input string) tea.Cmd {
	c, err := parseCommand(input)
	if err != nil {
		m.notice = err.Error()
		return nil
	}
	switch c.kind {
	case cmdView:
		return m.push(m.tabs[m.active].focus, c.view)
	case cmdHelp:
		m.mode = modeHelp
	case cmdQuit:
		return tea.Quit
	}
	return nil
}

// setFilter sets the filter of the view shown in pane id.
func (m *Model) setFilter(id layout.PaneID, query string) {
	s := m.stacks[id]
	s[len(s)-1] = s[len(s)-1].(views.Filterer).SetFilter(query)
}

// sidebarKey handles a key while the sidebar has focus.
func (m Model) sidebarKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() != "enter" {
		cmd := m.updateSidebar(msg)
		return m, cmd
	}
	cmd := m.openFromSidebar()
	return m, cmd
}

// openFromSidebar shows the stack of the selected sidebar node in the focused pane.
func (m *Model) openFromSidebar() tea.Cmd {
	stack, ok := m.sidebar.Stack()
	if !ok {
		return nil
	}
	m.stacks[m.tabs[m.active].focus] = stack
	m.focusSidebar(false)
	m.resizeViews()
	cmds := make([]tea.Cmd, len(stack))
	for i, v := range stack {
		cmds[i] = v.Init()
	}
	return tea.Batch(cmds...)
}

func (m *Model) updateSidebar(msg tea.Msg) tea.Cmd {
	v, cmd := m.sidebar.Update(msg)
	m.sidebar = v.(sidebar.Model)
	return cmd
}

func (m *Model) focusSidebar(f bool) {
	m.sidebarFocused = f
	m.sidebar = m.sidebar.SetFocused(f)
}

// sendTop passes msg to the view shown in pane id.
func (m *Model) sendTop(id layout.PaneID, msg tea.Msg) tea.Cmd {
	s := m.stacks[id]
	v, cmd := s[len(s)-1].Update(msg)
	s[len(s)-1] = v
	return cmd
}

// clickTop passes a click at (x, y) inside the view shown in pane id, if it takes clicks.
func (m *Model) clickTop(id layout.PaneID, x, y int) (tea.Cmd, bool) {
	s := m.stacks[id]
	c, ok := s[len(s)-1].(views.Clicker)
	if !ok {
		return nil, false
	}
	v, cmd, hit := c.Click(x, y)
	s[len(s)-1] = v
	return cmd, hit
}

// open pushes the view for the selected item onto the stack of pane id.
func (m *Model) open(id layout.PaneID) tea.Cmd {
	next, ok := m.top(id).Open()
	if !ok {
		return nil
	}
	return m.push(id, next)
}

// push shows v on top of the stack of pane id, moves focus from the sidebar to the panes and starts v.
func (m *Model) push(id layout.PaneID, v views.View) tea.Cmd {
	m.stacks[id] = append(m.stacks[id], v)
	m.focusSidebar(false)
	m.resizeViews()
	return v.Init()
}

// back pops the top view of pane id; the last view stays.
func (m *Model) back(id layout.PaneID) {
	if s := m.stacks[id]; len(s) > 1 {
		m.stacks[id] = s[:len(s)-1]
		m.resizeViews()
	}
}

// broadcast passes msg to the sidebar and every view of every stack of every tab.
func (m *Model) broadcast(msg tea.Msg) tea.Cmd {
	cmds := []tea.Cmd{m.updateSidebar(msg)}
	for _, t := range m.tabs {
		for _, id := range t.root.Panes() {
			s := m.stacks[id]
			for i, v := range s {
				var cmd tea.Cmd
				s[i], cmd = v.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}
	return tea.Batch(cmds...)
}

// runPrefix runs the action of key k after ctrl+w.
func (m Model) runPrefix(k string) (tea.Model, tea.Cmd) {
	if hjkl, ok := arrows[k]; ok {
		k = hjkl
	}
	t := &m.tabs[m.active]
	if m.sidebarFocused && !slices.Contains(sidebarPrefixKeys, k) {
		return m, nil
	}
	if m.sidebarFocused && k == "l" {
		m.focusSidebar(false)
		return m, nil
	}
	switch k {
	case "v", "s", "h", "j", "k", "l", "q", "=", "<", ">", "+", "-":
		t.zoomed = false
	}

	var cmd tea.Cmd
	if !m.layoutAction(t, k) && !m.focusAction(t, k) {
		cmd = m.tabAction(t, k)
	}
	m.resizeViews()
	return m, cmd
}

// layoutAction runs the prefix keys that split, close, zoom or resize panes;
// false when k is not one of them.
func (m *Model) layoutAction(t *tab, k string) bool {
	switch k {
	case "v":
		m.split(t, layout.Horizontal)
	case "s":
		m.split(t, layout.Vertical)
	case "q":
		m.closePane(t)
	case "z":
		t.zoomed = !t.zoomed
	case "=":
		t.root = t.root.Equalize()
	case ">":
		t.root = t.root.Resize(t.focus, layout.Horizontal, resizeStep)
	case "<":
		t.root = t.root.Resize(t.focus, layout.Horizontal, -resizeStep)
	case "+":
		t.root = t.root.Resize(t.focus, layout.Vertical, resizeStep)
	case "-":
		t.root = t.root.Resize(t.focus, layout.Vertical, -resizeStep)
	default:
		return false
	}
	return true
}

// focusAction runs the prefix keys that move focus between panes and the
// sidebar; false when k is not one of them.
func (m *Model) focusAction(t *tab, k string) bool {
	switch k {
	case "h":
		if !m.moveFocus(t, layout.Left) && m.sidebarDrawn() {
			m.focusSidebar(true)
		}
	case "j":
		m.moveFocus(t, layout.Down)
	case "k":
		m.moveFocus(t, layout.Up)
	case "l":
		m.moveFocus(t, layout.Right)
	case "e":
		m.sidebarVisible = !m.sidebarVisible
		m.focusSidebar(m.sidebarFocused && m.sidebarVisible)
	default:
		return false
	}
	return true
}

// tabAction runs the remaining prefix keys: new, rename, switch and help.
func (m *Model) tabAction(t *tab, k string) tea.Cmd {
	switch k {
	case "t":
		return m.newTab()
	case ",":
		m.startInput(modeRename, t.name)
	case "?":
		m.mode = modeHelp
	case "n":
		m.active = (m.active + 1) % len(m.tabs)
	case "p":
		m.active = (m.active + len(m.tabs) - 1) % len(m.tabs)
	default:
		if n, err := strconv.Atoi(k); err == nil && n >= 1 && n <= len(m.tabs) {
			m.active = n - 1
		}
	}
	return nil
}

func (m Model) sidebarDrawn() bool {
	return m.sidebarVisible && m.width >= sidebarWidth+minPaneWidth
}

func (m Model) sidebarRect() layout.Rect {
	return layout.Rect{X: 0, Y: 1, W: sidebarWidth, H: max(m.height-2, 0)}
}

func (m Model) paneArea() layout.Rect {
	area := layout.Rect{X: 0, Y: 1, W: m.width, H: max(m.height-2, 0)}
	if m.sidebarDrawn() {
		area.X, area.W = sidebarWidth, m.width-sidebarWidth
	}
	return area
}

// visible returns the rectangles of the panes drawn in the active tab.
func (m Model) visible() map[layout.PaneID]layout.Rect {
	t := m.tabs[m.active]
	if t.zoomed {
		return map[layout.PaneID]layout.Rect{t.focus: m.paneArea()}
	}
	return t.root.Rects(m.paneArea())
}

func (m *Model) resizeViews() {
	r := m.sidebarRect()
	m.sidebar = m.sidebar.Resize(max(r.W-2, 0), max(r.H-2, 0)).(sidebar.Model)
	for id, r := range m.visible() {
		s := m.stacks[id]
		s[len(s)-1] = s[len(s)-1].Resize(max(r.W-2, 0), max(r.H-2, 0))
	}
}

func (m *Model) split(t *tab, o layout.Orientation) {
	added := m.nextID
	root := t.root.SplitPane(t.focus, added, o)
	for _, r := range root.Rects(m.paneArea()) {
		if r.W < minPaneWidth || r.H < minPaneHeight {
			return
		}
	}
	m.nextID++
	m.stacks[added] = slices.Clone(m.stacks[t.focus])
	t.root, t.focus = root, added
}

// moveFocus focuses the neighbor pane in direction d; false if there is none.
func (m *Model) moveFocus(t *tab, d layout.Direction) bool {
	id, ok := t.root.Neighbor(t.focus, d, m.paneArea())
	if ok {
		t.focus = id
	}
	return ok
}

func (m *Model) closePane(t *tab) {
	if len(t.root.Panes()) == 1 {
		m.closeTab()
		return
	}
	closed := t.root.Rects(m.paneArea())[t.focus]
	t.root = t.root.Close(t.focus)
	delete(m.stacks, t.focus)
	t.focus = t.root.Panes()[0]
	for id, r := range t.root.Rects(m.paneArea()) {
		if r.X <= closed.X && closed.X < r.X+r.W && r.Y <= closed.Y && closed.Y < r.Y+r.H {
			t.focus = id
		}
	}
}

// closeTab removes the active tab and its views; the last tab stays.
func (m *Model) closeTab() {
	if len(m.tabs) == 1 {
		return
	}
	for _, id := range m.tabs[m.active].root.Panes() {
		delete(m.stacks, id)
	}
	m.tabs = slices.Delete(m.tabs, m.active, m.active+1)
	m.active = max(m.active-1, 0)
}

func (m *Model) newTab() tea.Cmd {
	id := m.nextID
	m.nextID++
	v := m.newView()
	m.stacks[id] = []views.View{v}
	m.tabs = slices.Insert(m.tabs, m.active+1, tab{root: layout.Leaf(id), focus: id})
	m.active++
	return v.Init()
}

// View draws the tab bar, panes, sidebar, status bar and help box.
func (m Model) View() tea.View {
	var content string
	if m.width > 0 && m.height > 0 {
		content = m.frame()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m Model) frame() string {
	comp := lipgloss.NewCompositor(
		lipgloss.NewLayer(m.tabBar()),
		lipgloss.NewLayer(m.statusBar()).Y(m.height-1),
	)
	focus := m.tabs[m.active].focus
	for id, r := range m.visible() {
		if !drawn(r) {
			continue
		}
		pane := renderPane(m.stacks[id], r, id == focus && !m.sidebarFocused)
		comp.AddLayers(lipgloss.NewLayer(pane).X(r.X).Y(r.Y))
	}
	if r := m.sidebarRect(); m.sidebarDrawn() && drawn(r) {
		pane := renderPane([]views.View{m.sidebar}, r, m.sidebarFocused)
		comp.AddLayers(lipgloss.NewLayer(pane).X(r.X).Y(r.Y))
	}
	if m.mode == modeHelp {
		if r := m.helpRect(); drawn(r) {
			box := renderBox([]string{"Help"}, strings.Join(helpLines(), "\n"), r, true)
			comp.AddLayers(lipgloss.NewLayer(box).X(r.X).Y(r.Y))
		}
	}
	if m.mode == modePalette {
		if r, ok := m.paletteRect(); ok {
			comp.AddLayers(lipgloss.NewLayer(m.paletteBox(r)).X(r.X).Y(r.Y))
		}
	}
	return lipgloss.NewCanvas(m.width, m.height).Compose(comp).Render()
}

// helpRect is the help box, centered in the window.
func (m Model) helpRect() layout.Rect {
	w := min(m.width-helpMargin, helpMaxWidth)
	h := min(m.height-helpMargin, len(helpLines())+2)
	return layout.Rect{X: (m.width - w) / 2, Y: (m.height - h) / 2, W: w, H: h}
}

func (m Model) tabBar() string {
	var b strings.Builder
	for i := range m.tabs {
		label := m.tabLabel(i)
		if i == m.active {
			label = reverseStyle.Render(label)
		}
		b.WriteString(label)
	}
	return lipgloss.NewStyle().MaxWidth(m.width).Render(b.String())
}

// tabLabel is tab i as drawn in the tab bar, without the active mark.
func (m Model) tabLabel(i int) string {
	t := m.tabs[i]
	name := t.name
	if name == "" {
		name = m.top(t.focus).Title()
	}
	return " " + strconv.Itoa(i+1) + " " + name + " "
}

func (m Model) statusBar() string {
	var bar string
	switch m.mode {
	case modeFilter:
		bar = m.inputLine("/") + "  " + hintStyle.Render(filterHints)
	case modePalette:
		bar = hintStyle.Render(paletteHints)
	case modeRename:
		bar = m.inputLine(renamePrompt) + "  " + hintStyle.Render(renameHints)
	default:
		bar = m.modeBar()
	}
	return lipgloss.NewStyle().MaxWidth(m.width).Render(bar)
}

// inputLine draws prompt and the input, as wide as its text, up to the window.
// A long text scrolls and keeps the cursor in view.
func (m Model) inputLine(prompt string) string {
	in := m.input
	// One cell stays free for the cursor at the end of the text.
	in.SetWidth(min(lipgloss.Width(in.Value())+1, max(m.width-lipgloss.Width(prompt)-1, 1)))
	// The stored input has no width, so it never scrolled: rebuild the
	// window from the end, then move it back to the cursor.
	pos := in.Position()
	in.CursorEnd()
	in.SetCursor(pos)
	return prompt + in.View()
}

// modeBar is the status bar outside the input modes: badge, path or notice, hints.
// modeBar is the badge, a notice when there is one, and the hints; the pane
// border already shows the path.
func (m Model) modeBar() string {
	badge, hints := " NORMAL ", normalHints
	if m.sidebarFocused {
		hints = sidebarHints
	}
	switch {
	case m.mode == modePrefix && m.sidebarFocused:
		badge, hints = " CTRL-W ", sidebarPrefixHints
	case m.mode == modePrefix:
		badge, hints = " CTRL-W ", prefixHints
	}
	bar := reverseStyle.Render(badge)
	if m.notice != "" {
		bar += " " + errorStyle.Render(m.notice)
	}
	return bar + "  " + hintStyle.Render(hints)
}

func titles(stack []views.View) []string {
	t := make([]string, len(stack))
	for i, v := range stack {
		t[i] = v.Title()
	}
	return t
}

// fitPath joins path segments to fit width cells. A path that is too long
// loses leading segments, marked with "… › "; the last segment alone is cut.
func fitPath(path []string, width int) string {
	if len(path) == 0 || width <= 0 {
		return ""
	}
	if s := strings.Join(path, pathSep); lipgloss.Width(s) <= width {
		return s
	}
	for i := 1; i < len(path); i++ {
		if s := pathCut + strings.Join(path[i:], pathSep); lipgloss.Width(s) <= width {
			return s
		}
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(path[len(path)-1])
}

// renderPane draws the top view of stack inside a border, with the path of
// the stack in the top border. The result is exactly r.W x r.H cells; r must
// be at least 2x2.
func renderPane(stack []views.View, r layout.Rect, focused bool) string {
	return renderBox(titles(stack), stack[len(stack)-1].Render(), r, focused)
}

// renderBox draws body inside a border, with path in the top border.
func renderBox(path []string, body string, r layout.Rect, focused bool) string {
	color := borderColor
	title := lipgloss.NewStyle()
	if focused {
		color = focusColor
		title = title.Foreground(focusColor).Bold(true)
	}
	edge := lipgloss.NewStyle().Foreground(color)

	inner := r.W - 2
	label := ""
	if inner > 0 {
		label = title.MaxWidth(inner).Render(" " + fitPath(path, inner-2) + " ")
	}
	top := edge.Render(border.TopLeft) + label +
		edge.Render(strings.Repeat(border.Top, inner-lipgloss.Width(label))+border.TopRight)
	bottom := edge.Render(border.BottomLeft + strings.Repeat(border.Bottom, inner) + border.BottomRight)

	rows := strings.Split(body, "\n")
	fit := lipgloss.NewStyle().MaxWidth(inner)
	side := edge.Render(border.Left)
	lines := []string{top}
	for i := range r.H - 2 {
		line := ""
		if i < len(rows) && inner > 0 {
			line = fit.Render(rows[i])
		}
		line += strings.Repeat(" ", inner-lipgloss.Width(line))
		lines = append(lines, side+line+edge.Render(border.Right))
	}
	return strings.Join(append(lines, bottom), "\n")
}
