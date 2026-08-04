// The bubbletea stepper for `crust debug`. The recording it walks is
// built in internal/debugger (pure Go, no terminal dependency at
// all — that's where the real logic and its test coverage live); this
// file is layout and key handling.
//
// Two tabs, switched with tab/left/right: KPIs (two pie charts — time
// and memory per function/loop — plus a few overall numbers) and
// Stepper (the recorded tree, navigable with the arrow keys).
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

// tab is which of the two panes is showing.
type tab int

const (
	tabKPI tab = iota
	tabStepper
)

// visRow is one visible line of the stepper's tree: a node plus its
// indentation depth.
type visRow struct {
	node  *debugger.TraceNode
	depth int
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
func buildRows(nodes []*debugger.TraceNode, depth int, expanded map[*debugger.TraceNode]bool) []visRow {
	var rows []visRow
	for _, n := range nodes {
		rows = append(rows, visRow{n, depth})
		if n.Folded && !expanded[n] {
			continue
		}
		rows = append(rows, buildRows(n.Children, depth+1, expanded)...)
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
	}
	return m, nil
}

func (m debugModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		return m, tea.Quit
	case "tab", "right", "l":
		m.active = (m.active + 1) % 2
	case "shift+tab", "left", "h":
		m.active = (m.active + 2 - 1) % 2
	case "down", "j":
		if m.active == tabStepper {
			m.moveCursor(1)
		}
	case "up", "k":
		if m.active == tabStepper {
			m.moveCursor(-1)
		}
	case "enter", " ":
		if m.active == tabStepper {
			m.toggleFold()
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
	if m.active == tabKPI {
		body = m.viewKPI()
	} else {
		body = m.viewStepper()
	}

	var b strings.Builder
	b.WriteString(m.viewTabs())
	b.WriteByte('\n')
	b.WriteString(m.view.header())
	b.WriteByte('\n')
	b.WriteString(body)
	b.WriteByte('\n')
	b.WriteString(styleHelp.Render(m.helpText()))
	return b.String()
}

func (m debugModel) helpText() string {
	if m.active == tabKPI {
		return "tab/←→: switch tab   q: quit"
	}
	return "tab/←→: switch tab   ↑↓: move   enter: open/close   q: quit"
}

func (m debugModel) viewTabs() string {
	kpi, stepper := styleTabIdle, styleTabIdle
	if m.active == tabKPI {
		kpi = styleTabActive
	} else {
		stepper = styleTabActive
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, kpi.Render("KPIs"), stepper.Render("Stepper"))
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
		if r.depth == 0 {
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

// runDebugTUI drives the stepper on a real terminal.
func runDebugTUI(view *debugView, stdin io.Reader, stdout, stderr io.Writer) int {
	m := newDebugModel(view)
	opts := []tea.ProgramOption{tea.WithOutput(stdout)}
	if f, ok := stdin.(*os.File); ok {
		opts = append(opts, tea.WithInput(f))
	}
	prog := tea.NewProgram(m, opts...)
	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(stderr, "crust debug: %v\n", err)
		return 1
	}
	return 0
}
