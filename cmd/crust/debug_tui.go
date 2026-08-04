// The bubbletea stepper for `crust debug`. The recording it walks is
// built in internal/debugger (pure Go, no terminal dependency at
// all — that's where the real logic and its test coverage live); this
// file is layout and key handling.
//
// Three tabs, switched with tab/left/right: KPIs (two pie charts — time
// and memory per function/loop — plus a few overall numbers), Stepper
// (the recorded tree, navigable with the arrow keys), and Editor (hands
// the terminal to nvim on the file being debugged; see debug_editor.go
// for the save-triggered reload loop).
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Sintfoap/cRust/internal/debugger"
)

// tab is which of the three panes is showing.
type tab int

const (
	tabKPI tab = iota
	tabStepper
	tabEditor
)

// tabCount is how many tabs there are, for wrapping cursor arithmetic —
// named rather than inlined as a literal 3 so handleKey's wraparound
// math stays correct if a fourth tab ever shows up.
const tabCount = 3

// visRow is one visible line of the stepper's tree: a node plus its
// indentation depth. closing marks a synthetic row generated after a
// node's children (see buildRows) rather than the node's own row —
// two visRows can share the same node, one opening and one closing.
type visRow struct {
	node    *debugger.TraceNode
	depth   int
	closing bool
}

// debugModel is the TUI's state.
type debugModel struct {
	view   *debugView
	active tab
	width  int
	height int

	// expanded tracks only folded rows (see buildRows) — every other
	// row always shows its children, so there's nothing else to track.
	// Fold rows default collapsed (that's the whole point of folding a
	// thousand-lap loop), so an empty map is the right zero value.
	expanded map[*debugger.TraceNode]bool
	rows     []visRow
	cursor   int
	top      int // first visible row, for scrolling a tall tree

	// opts and stdin are only needed for the Editor tab's reload loop
	// (debug_editor.go) — a rerun after a save must record the file
	// with the exact same --store/--max-steps the original run used,
	// and stdin so an input()-reading program still has something to
	// read from. runDebugTUI sets both after construction; tests that
	// never touch the Editor tab can leave them at their zero values.
	opts  debugOptions
	stdin io.Reader

	// editorErr is the last nvim launch/exit error, if any, shown on
	// the Editor tab — e.g. nvim isn't on PATH. Cleared on success.
	editorErr string
}

func newDebugModel(view *debugView) debugModel {
	m := debugModel{view: view, expanded: map[*debugger.TraceNode]bool{}}
	m.rebuildRows()
	return m
}

func (m *debugModel) rebuildRows() {
	m.rows = buildRows(m.view.rec.Roots(), 0, m.expanded)
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// buildRows flattens the recorded tree into visible rows, respecting
// which folded rows are open. Only folded rows ever hide children —
// an ordinary frame or step always shows everything under it, since
// the fold mechanism (recorder.go) is already what keeps a huge run
// from flooding the view; a second, general expand/collapse on top of
// that would just be more UI for the same job.
//
// Every node whose children actually get shown also gets a synthetic
// "// end ..." row right after them, at the same depth as its own
// opening row — a long recipe call or loop's body can run for dozens
// of screen rows, and without a closing marker there's nothing to scan
// for besides indentation to tell where it ends, the same problem
// unbraced code would have.
func buildRows(nodes []*debugger.TraceNode, depth int, expanded map[*debugger.TraceNode]bool) []visRow {
	var rows []visRow
	for _, n := range nodes {
		rows = append(rows, visRow{node: n, depth: depth})
		if n.Folded && !expanded[n] {
			continue
		}
		if len(n.Children) == 0 {
			continue
		}
		rows = append(rows, buildRows(n.Children, depth+1, expanded)...)
		rows = append(rows, visRow{node: n, depth: depth, closing: true})
	}
	return rows
}

func (m debugModel) Init() tea.Cmd { return nil }

func (m debugModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case nvimExitMsg:
		return m.handleNvimExit(msg)
	case reloadMsg:
		return m.handleReload(msg)
	}
	return m, nil
}

func (m debugModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		return m, tea.Quit
	case "tab", "right", "l":
		m.active = (m.active + 1) % tabCount
	case "shift+tab", "left", "h":
		m.active = (m.active + tabCount - 1) % tabCount
	case "down", "j":
		if m.active == tabStepper {
			m.moveCursor(1)
		}
	case "up", "k":
		if m.active == tabStepper {
			m.moveCursor(-1)
		}
	case "enter", " ":
		switch m.active {
		case tabStepper:
			m.toggleFold()
		case tabEditor:
			return m, m.openEditorCmd()
		}
	}
	return m, nil
}

func (m *debugModel) moveCursor(delta int) {
	if len(m.rows) == 0 {
		return
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	visible := m.stepperBodyHeight()
	if m.cursor < m.top {
		m.top = m.cursor
	}
	if m.cursor >= m.top+visible {
		m.top = m.cursor - visible + 1
	}
}

func (m *debugModel) toggleFold() {
	if len(m.rows) == 0 {
		return
	}
	n := m.rows[m.cursor].node
	if !n.Folded {
		return
	}
	m.expanded[n] = !m.expanded[n]
	m.rebuildRows()
}

// stepperBodyHeight is how many tree rows fit on screen at once, below
// the tab bar, the column header, and above the help footer.
func (m debugModel) stepperBodyHeight() int {
	h := m.height - 6
	if h < 3 {
		h = 3
	}
	return h
}

func (m debugModel) View() string {
	var body string
	switch m.active {
	case tabKPI:
		body = m.viewKPI()
	case tabStepper:
		body = m.viewStepper()
	case tabEditor:
		body = m.viewEditor()
	}

	var b strings.Builder
	b.WriteString(m.viewTabs())
	b.WriteByte('\n')
	b.WriteString(m.view.header())
	b.WriteByte('\n')
	b.WriteString(body)
	b.WriteByte('\n')
	b.WriteString(styleHelp.Render(m.helpText()))
	return clampHeight(b.String(), m.height)
}

// clampHeight trims s to at most n lines when n > 0, replacing
// whatever's left over with a one-line note. The stepper already
// clamps its own body to stepperBodyHeight, but the KPI tab's two pie
// charts don't scale down for a small terminal — without this, their
// full (uncapped) height flows past the bottom of the window and the
// terminal's own scrolling (there's no fixed scroll region) carries
// the tab bar and header, printed first, right off the top with it.
// Clamping the whole frame to the window's actual height keeps those
// first lines on screen no matter how tall a tab's content gets.
func clampHeight(s string, n int) string {
	if n <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	lines = lines[:n-1]
	lines = append(lines, styleFaint.Render("… (grow the terminal to see the rest)"))
	return strings.Join(lines, "\n")
}

func (m debugModel) helpText() string {
	switch m.active {
	case tabStepper:
		return "tab/←→: switch tab   ↑↓: move   enter: open/close   q: quit"
	case tabEditor:
		return "tab/←→: switch tab   enter: open in nvim (saving reruns + reopens)   q: quit"
	default:
		return "tab/←→: switch tab   q: quit"
	}
}

func (m debugModel) viewTabs() string {
	kpi, stepper, editor := styleTabIdle, styleTabIdle, styleTabIdle
	switch m.active {
	case tabKPI:
		kpi = styleTabActive
	case tabStepper:
		stepper = styleTabActive
	case tabEditor:
		editor = styleTabActive
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, kpi.Render("KPIs"), stepper.Render("Stepper"), editor.Render("Editor"))
}

// viewEditor is the Editor tab's placeholder. It hands off to a real
// nvim (debug_editor.go) rather than drawing an editor pane inline —
// embedding one would mean shipping a full terminal emulator inside
// this TUI just to render nvim's own screen, a much bigger and more
// fragile build than reusing a real editor wholesale — so there's
// nothing to render here but instructions and whatever the last
// attempt to launch nvim came back with.
func (m debugModel) viewEditor() string {
	lines := []string{
		fmt.Sprintf("file: %s", m.view.path),
		"",
		"enter: open it in nvim",
		"saving (:w) reruns the file, refreshes the KPI/Stepper tabs, and reopens nvim",
		"quitting without saving (:q) returns you here",
	}
	if m.editorErr != "" {
		lines = append(lines, "", styleError.Render("last attempt failed: "+m.editorErr))
	}
	return styleMuted.Render(strings.Join(lines, "\n"))
}

// viewKPI is the time and memory pie charts, side by side when there's
// room, plus a few overall numbers a pie chart can't show on its own
// (step count, whether the recording was capped, the single slowest
// statement) — the "whatever other KPIs might be useful" the user
// asked for, kept to a short list rather than a whole extra tab.
func (m debugModel) viewKPI() string {
	kpis := m.view.timing().KPIs()

	timeChart := pieChart(kpis, 7, func(k debugger.KPI) float64 { return float64(k.SelfTime) },
		func(k debugger.KPI) string { return k.SelfTime.String() })
	sizeChart := pieChart(kpis, 7, func(k debugger.KPI) float64 { return float64(k.SelfSize) },
		func(k debugger.KPI) string { return fmt.Sprintf("%d", k.SelfSize) })

	timeCol := lipgloss.JoinVertical(lipgloss.Left, styleTitle.Render("time by function"), timeChart)
	sizeCol := lipgloss.JoinVertical(lipgloss.Left, styleTitle.Render("memory by function"), sizeChart)

	var charts string
	if m.width > 0 && m.width < 70 {
		charts = lipgloss.JoinVertical(lipgloss.Left, timeCol, "", sizeCol)
	} else {
		charts = lipgloss.JoinHorizontal(lipgloss.Top, timeCol, "   ", sizeCol)
	}

	return charts + "\n\n" + m.viewOverallStats()
}

// viewOverallStats is the handful of numbers that don't fit naturally
// into a per-function breakdown: how big the run was, whether the
// recorder had to stop early, and which single statement cost the most
// on its own (as opposed to which *family* did — a KPI bucket answers
// "which function", this answers "which line").
func (m debugModel) viewOverallStats() string {
	rec := m.view.rec
	t := m.view.timing()
	lines := []string{
		fmt.Sprintf("steps recorded: %d%s", rec.Steps(), truncatedNote(rec)),
		fmt.Sprintf("total time: %s", t.Overall()),
	}
	if slow, ok := slowestStep(m.rows, t); ok {
		lines = append(lines, fmt.Sprintf("slowest statement: %s (%s self)", slow.Label(), t.Of(slow).Self))
	}
	return styleMuted.Render(strings.Join(lines, "\n"))
}

func truncatedNote(rec *debugger.Recorder) string {
	if rec.Truncated() {
		return " (capped)"
	}
	return ""
}

// slowestStep finds the single step (not frame) with the highest self
// time across every row currently known (including inside collapsed
// folds — a slow statement doesn't stop being the answer to "what's
// slow" just because its lap happens to be folded away).
func slowestStep(rows []visRow, t *debugger.Timing) (*debugger.TraceNode, bool) {
	var best *debugger.TraceNode
	var bestSelf int64
	var walk func(n *debugger.TraceNode)
	walk = func(n *debugger.TraceNode) {
		if !n.IsFrame() {
			if self := int64(t.Of(n).Self); best == nil || self > bestSelf {
				best, bestSelf = n, self
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, r := range rows {
		if r.depth == 0 && !r.closing {
			walk(r.node)
		}
	}
	if best == nil {
		return nil, false
	}
	return best, true
}

// viewStepper is the scrollable tree table: the same columns
// writePlain prints, but interactive, with the cursor row highlighted.
func (m debugModel) viewStepper() string {
	if len(m.rows) == 0 {
		return styleMuted.Render("(nothing recorded)")
	}
	t := m.view.timing()

	var b strings.Builder
	b.WriteString(styleFaint.Render(fmt.Sprintf("%s %s %6s %9s %7s\n",
		col("step", 40), col("out", 18), "size", "time", "self%")))

	visible := m.stepperBodyHeight()
	end := m.top + visible
	if end > len(m.rows) {
		end = len(m.rows)
	}
	for i := m.top; i < end; i++ {
		b.WriteString(m.renderRow(i, t))
		b.WriteByte('\n')
	}
	return b.String()
}

func (m debugModel) renderRow(i int, t *debugger.Timing) string {
	row := m.rows[i]
	if row.closing {
		return m.renderClosingRow(i)
	}
	n := row.node
	indent := strings.Repeat("  ", row.depth)
	nt := t.Of(n)
	label, out, size := indent+n.Label(), "", ""
	if !n.IsFrame() {
		out = shortInspect(n.Step.Out)
		size = sizeText(n.Step)
	}
	line := fmt.Sprintf("%s %s %6s %9s %7s",
		col(label, 40), col(out, 18), size, nt.Total.String(), selfPctText(nt, t.Overall()))

	style := lipgloss.NewStyle()
	switch {
	case !n.IsFrame() && n.Step.Failed():
		style = styleError
	case n.IsFrame():
		style = styleFaint
	}
	if i == m.cursor {
		return styleSelectedRow.Render("> " + line)
	}
	return "  " + style.Render(line)
}

// renderClosingRow draws the synthetic "// end ..." marker buildRows
// inserts after a node's children — the same node as its opening row,
// so pressing enter/space on a closing row folds a foldable node back
// up exactly like pressing it on the opening row would.
func (m debugModel) renderClosingRow(i int) string {
	row := m.rows[i]
	line := strings.Repeat("  ", row.depth) + "// end " + closingLabel(row.node.Label())
	if i == m.cursor {
		return styleSelectedRow.Render("> " + line)
	}
	return "  " + styleFaint.Render(line)
}

// runDebugTUI drives the stepper on a real terminal. AltScreen keeps
// the whole session inside the terminal's alternate buffer (restored
// to the normal buffer and scrollback on exit) rather than scrolling
// the ordinary window as frames redraw — the standard choice for a
// full-screen app like this one, and it pairs with View's clampHeight
// call to keep the tab bar on screen regardless of window size.
func runDebugTUI(view *debugView, opts debugOptions, stdin io.Reader, stdout, stderr io.Writer) int {
	m := newDebugModel(view)
	m.opts = opts
	m.stdin = stdin
	progOpts := []tea.ProgramOption{tea.WithOutput(stdout), tea.WithAltScreen()}
	if f, ok := stdin.(*os.File); ok {
		progOpts = append(progOpts, tea.WithInput(f))
	}
	prog := tea.NewProgram(m, progOpts...)
	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(stderr, "crust debug: %v\n", err)
		return 1
	}
	return 0
}
