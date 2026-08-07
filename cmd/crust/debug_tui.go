// The bubbletea stepper for `crust develop`. The recording it walks is
// built in internal/debugger (pure Go, no terminal dependency at
// all — that's where the real logic and its test coverage live); this
// file is layout and key handling.
//
// Six tabs, switched with tab/left/right: Time and Memory (one pie
// chart each, self-time/self-memory per function/loop, plus a few
// overall numbers), Stepper (the recorded tree, navigable with the
// arrow keys), Editor (hands the terminal to nvim on the file being
// debugged; see debug_editor.go for the save-triggered reload loop),
// Run (type a path and press enter to run the file with it as stdin
// and see the raw output, command-line style; see debug_run.go), and
// Files (browse and switch to another .crust file alongside the one
// currently open; see debug_nav.go).
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

// tab is which of the six panes is showing.
type tab int

const (
	tabTime tab = iota
	tabMemory
	tabStepper
	tabEditor
	tabRun
	// tabNav is appended last, after every pre-existing tab, so none of
	// their const values (or anything that stores one, like a saved
	// m.active) shift out from under it.
	tabNav
)

// tabCount is how many tabs there are, for wrapping cursor arithmetic —
// named rather than inlined as a literal so handleKey's wraparound math
// stays correct if another tab ever shows up.
const tabCount = 6

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

	// opts is only needed for the Editor tab's reload loop
	// (debug_editor.go) — a rerun after a save must record the file
	// with the exact same --store/--max-steps the original run used.
	// runDebugTUI sets it after construction; tests that never touch
	// the Editor tab can leave it at its zero value. reloadCmd reads
	// its input file from m.runInput, not from opts or any process
	// stdin — see emptyDebugView (debug.go) for why the interactive
	// TUI never touches real stdin for tracing at all.
	opts debugOptions

	// editorErr is the last nvim launch/exit error, if any, shown on
	// the Editor tab — e.g. nvim isn't on PATH. Cleared on success.
	editorErr string

	// runInput/runOutput/runFailed back the Run tab (debug_run.go):
	// runInput is the input-file path field, runOutput is the raw
	// stdout+stderr text from the last run (or an error opening the
	// input file), and runFailed is whether that run's exit code was
	// non-zero, purely to decide whether runOutput renders as an error.
	runInput  runInputModel
	runOutput string
	runFailed bool

	// runEntryIndex/runEntryFocused back the Run tab's entry-point
	// selector: which of m.view.entryPoints() is picked, and whether
	// up/down has moved focus to that row (false = the input-file
	// field has focus). runDebugTUI/handleReload/handleNvimExit set
	// runEntryIndex to match m.opts.Store right after computing a fresh
	// view (or, for handleNvimExit, a rescanned entry list on the
	// existing one), so the selector starts pointed at whichever entry
	// point the rest of the TUI is already showing.
	runEntryIndex   int
	runEntryFocused bool

	// runAllStores toggles the Run tab (Ctrl+R) between running only
	// selectedRunEntry() (the ordinary behavior) and running every
	// entry point m.runEntryOptions() reports, each against the same
	// input file, in sequence — see runAllStoresCmd (debug_run.go).
	// Reserved to a control key rather than a typed letter specifically
	// so it's never ambiguous with typing into the input-file field
	// (see handleRunTabKey's own doc comment on which keys stay
	// reserved there and why).
	runAllStores bool

	// navFiles/navCursor back the Files tab (debug_nav.go): every
	// *.crust file alongside the one currently open (including it),
	// sorted, and which one navCursor currently points at. Rescanned on
	// every tab switch that lands on Files (maybeRefreshNav) rather than
	// cached across the whole session — directory contents can change
	// between visits, and an os.ReadDir is cheap enough not to bother
	// caching. navErr holds the last switch attempt's parse error, if
	// any, so a broken target file doesn't disturb the still-valid
	// current view (the same "leave the last good state alone" choice
	// editorErr already makes for a failed nvim launch).
	navFiles  []string
	navCursor int
	navErr    string

	// navCreating/navNewName back the Files tab's "new file" prompt
	// (debug_nav.go's createNavFile): navCreating switches the tab from
	// the file list to a single text field for the new name, reusing
	// runInputModel (the Run tab's own hand-rolled field) rather than a
	// second implementation of the same small job.
	navCreating bool
	navNewName  runInputModel
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
	case runResultMsg:
		return m.handleRunResult(msg)
	}
	return m, nil
}

func (m debugModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// The Run tab's input field wants almost every key for itself
	// (including letters bound to actions everywhere else, like q for
	// quit or h/j/k/l for movement, since a file path can contain any
	// of them), so it gets a completely separate handler rather than a
	// case or two threaded through the switch below.
	if m.active == tabRun {
		return m.handleRunTabKey(msg)
	}
	// Same reasoning as the Run tab's own field above: typing a new
	// filename wants nearly every key for itself (including letters
	// that mean something elsewhere, like q or j/k), so it gets its own
	// handler the moment the prompt is up rather than a case threaded
	// through the switch below.
	if m.active == tabNav && m.navCreating {
		return m.handleNavCreateKey(msg)
	}
	switch msg.String() {
	case "q", "ctrl+c", "esc":
		return m, tea.Quit
	case "tab", "right", "l":
		m.active = (m.active + 1) % tabCount
		m.maybeRefreshNav()
		return m, m.maybeOpenEditor()
	case "shift+tab", "left", "h":
		m.active = (m.active + tabCount - 1) % tabCount
		m.maybeRefreshNav()
		return m, m.maybeOpenEditor()
	case "down", "j":
		if m.active == tabStepper {
			m.moveCursor(1)
		} else if m.active == tabNav {
			m.moveNavCursor(1)
		}
	case "up", "k":
		if m.active == tabStepper {
			m.moveCursor(-1)
		} else if m.active == tabNav {
			m.moveNavCursor(-1)
		}
	case "enter", " ":
		switch m.active {
		case tabStepper:
			m.toggleFold()
		case tabEditor:
			return m, m.openEditorCmd()
		case tabNav:
			m = m.switchToSelectedFile()
		}
	case "n":
		if m.active == tabNav {
			m.navCreating = true
			m.navNewName = runInputModel{}
			m.navErr = ""
		}
	}
	return m, nil
}

// maybeOpenEditor launches nvim the moment a tab switch lands on the
// Editor tab, so there's no separate "now press enter" step once
// you're actually there — switching to the tab is the open action.
// Returns nil for every other tab, so tab/shift-tab stays a no-op cmd
// everywhere else, exactly as before this existed.
func (m debugModel) maybeOpenEditor() tea.Cmd {
	if m.active != tabEditor {
		return nil
	}
	return m.openEditorCmd()
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
	case tabTime:
		body = m.viewTime()
	case tabMemory:
		body = m.viewMemory()
	case tabStepper:
		body = m.viewStepper()
	case tabEditor:
		body = m.viewEditor()
	case tabRun:
		body = m.viewRun()
	case tabNav:
		body = m.viewNav()
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
// clamps its own body to stepperBodyHeight, but the Time/Memory tabs'
// pie charts don't scale down for a small terminal — without this,
// their full (uncapped) height flows past the bottom of the window and
// the terminal's own scrolling (there's no fixed scroll region) carries
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
		return "enter: reopen nvim   tab/←→: switch tab   q: quit"
	case tabRun:
		if len(m.runEntryOptions()) > 0 {
			return "↑↓: field/entry point   ←→: move/change   enter: run   tab/⇧tab: switch tab   ctrl+c/esc: quit"
		}
		return "enter: run   tab/⇧tab: switch tab   ctrl+c/esc: quit"
	case tabNav:
		if m.navCreating {
			return "enter: create and switch to it   esc: cancel   ctrl+c: quit"
		}
		return "tab/←→: switch tab   ↑↓: move   enter: switch to this file   n: new file   q: quit"
	default:
		return "tab/←→: switch tab   q: quit"
	}
}

func (m debugModel) viewTabs() string {
	timeS, memS, stepper, editor, run, nav := styleTabIdle, styleTabIdle, styleTabIdle, styleTabIdle, styleTabIdle, styleTabIdle
	switch m.active {
	case tabTime:
		timeS = styleTabActive
	case tabMemory:
		memS = styleTabActive
	case tabStepper:
		stepper = styleTabActive
	case tabEditor:
		editor = styleTabActive
	case tabRun:
		run = styleTabActive
	case tabNav:
		nav = styleTabActive
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		timeS.Render("Time"), memS.Render("Memory"), stepper.Render("Stepper"),
		editor.Render("Editor"), run.Render("Run"), nav.Render("Files"))
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
		"nvim opens automatically whenever you switch to this tab (enter reopens it too)",
		"nvim behaves normally in there: :w saves and keeps editing",
		"quitting after a save (:wq, :x, ZZ, ...) reruns the file and takes you to the Time tab",
		"quitting without ever saving (:q) just returns you here",
	}
	if m.editorErr != "" {
		lines = append(lines, "", styleError.Render("last attempt failed: "+m.editorErr))
	}
	return styleMuted.Render(strings.Join(lines, "\n"))
}

// viewTime is the self-time pie chart plus the overall time numbers a
// chart can't show on its own (step count, whether the recording was
// capped, the single slowest statement) — the "whatever other KPIs
// might be useful" the user originally asked for, kept to a short list
// rather than yet another tab. Split from viewMemory (below) into its
// own tab specifically so each chart gets the full window rather than
// squeezing two into one, per direct user feedback.
func (m debugModel) viewTime() string {
	kpis := m.view.timing().KPIs()
	chart := pieChart(kpis, 7, func(k debugger.KPI) float64 { return float64(k.SelfTime) },
		func(k debugger.KPI) string { return k.SelfTime.String() })
	col := lipgloss.JoinVertical(lipgloss.Left, styleTitle.Render("time by function"), chart)
	return col + "\n\n" + m.viewTimeStats()
}

// viewMemory is viewTime's counterpart for self-size — same chart
// function, same overall-numbers idea, but sized/labeled for the
// question "where did the memory go" instead of "where did the time
// go" (largest single value in place of slowest statement, since
// duration doesn't apply to a memory reading).
func (m debugModel) viewMemory() string {
	kpis := m.view.timing().KPIs()
	chart := pieChart(kpis, 7, func(k debugger.KPI) float64 { return float64(k.SelfSize) },
		func(k debugger.KPI) string { return fmt.Sprintf("%d", k.SelfSize) })
	col := lipgloss.JoinVertical(lipgloss.Left, styleTitle.Render("memory by function"), chart)
	return col + "\n\n" + m.viewMemoryStats()
}

// viewTimeStats is the handful of time-related numbers that don't fit
// naturally into a per-function breakdown: how big the run was,
// whether the recorder had to stop early, and which single statement
// cost the most on its own (as opposed to which *family* did — a KPI
// bucket answers "which function", this answers "which line").
func (m debugModel) viewTimeStats() string {
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

// viewMemoryStats mirrors viewTimeStats for the Memory tab: step count
// (there's no single "total memory" the way there's a total time, since
// sizes of different values don't sum into one meaningful number) plus
// the single biggest value recorded anywhere in the run.
func (m debugModel) viewMemoryStats() string {
	rec := m.view.rec
	lines := []string{
		fmt.Sprintf("steps recorded: %d%s", rec.Steps(), truncatedNote(rec)),
	}
	if big, ok := largestValue(m.rows); ok {
		lines = append(lines, fmt.Sprintf("largest single value: %s (size %d)", big.Label(), big.Step.Size))
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

// largestValue is slowestStep's counterpart for the Memory tab: the
// single step (not frame) with the biggest recorded value size across
// every row currently known, including inside collapsed folds. Steps
// whose size wasn't captured (SizeOK false — a scalar, see
// trace.SizeOf) never win, since there's nothing to compare.
func largestValue(rows []visRow) (*debugger.TraceNode, bool) {
	var best *debugger.TraceNode
	var bestSize int
	var walk func(n *debugger.TraceNode)
	walk = func(n *debugger.TraceNode) {
		if !n.IsFrame() && n.Step.SizeOK {
			if best == nil || n.Step.Size > bestSize {
				best, bestSize = n, n.Step.Size
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

// stdinNoNamer wraps *os.File to expose Read/Write/Close/Fd but not
// Name, threading the needle between two lookalike-but-different
// interfaces bubbletea's dependencies use to type-assert their way to
// "this is a real terminal file":
//
//   - github.com/muesli/cancelreader's File requires Name() too. On
//     Linux, satisfying it makes cancelreader take an epoll-based
//     reader — EPOLL_CTL_ADD on stdin's fd, which under some WSL
//     configurations fails outright ("add reader to epoll interest
//     list"), crashing `crust develop` before the TUI ever draws a
//     frame. Not satisfying it falls back to a plain blocking Read,
//     which works everywhere epoll doesn't, at the cost of being
//     unable to interrupt an in-progress read from the *outside* — a
//     real tradeoff in general, but not one `crust develop` ever
//     needed: every quit path here (q/ctrl+c/esc, or handing off to
//     nvim) is itself the next keypress being read, not an external
//     cancellation racing a read that's already blocked waiting for
//     one.
//   - github.com/charmbracelet/x/term's File does NOT require Name —
//     just Fd() plus the usual read/write/close. bubbletea's own
//     initInput uses exactly this narrower assertion to decide whether
//     to call term.MakeRaw and put the terminal in raw mode (no local
//     echo, no line buffering, one keypress in as one event out). An
//     earlier version of this wrapper (stdinOnlyReader) hid Fd()
//     entirely to be rid of cancelreader's epoll path, but that
//     defeated *this* check too — raw mode never engaged, so every
//     keystroke got echoed straight into the terminal by the OS
//     instead of being consumed by bubbletea, corrupting the TUI's own
//     rendered output with whatever the user happened to be typing.
//
// A named (non-embedded) *os.File field is what makes this possible:
// embedding would auto-promote Name() right along with everything
// else. Explicit forwarding methods for exactly the four wanted is the
// only way to expose that subset and no more.
type stdinNoNamer struct{ f *os.File }

func (r stdinNoNamer) Read(p []byte) (int, error)  { return r.f.Read(p) }
func (r stdinNoNamer) Write(p []byte) (int, error) { return r.f.Write(p) }
func (r stdinNoNamer) Close() error                { return r.f.Close() }
func (r stdinNoNamer) Fd() uintptr                 { return r.f.Fd() }

// runDebugTUI drives the stepper on a real terminal. AltScreen keeps
// the whole session inside the terminal's alternate buffer (restored
// to the normal buffer and scrollback on exit) rather than scrolling
// the ordinary window as frames redraw — the standard choice for a
// full-screen app like this one, and it pairs with View's clampHeight
// call to keep the tab bar on screen regardless of window size.
// m.runInput is pre-filled from this file's remembered settings
// (debug_state.go's restoreRunInput), same as opts.Store already
// reflects them via runDebug's applySavedStore. m.refreshNavFiles
// populates the Files tab's listing up front too, rather than waiting
// for the first tab switch that lands on it, so it's never seen empty
// due to simply not having been visited yet.
func runDebugTUI(view *debugView, opts debugOptions, stdin io.Reader, stdout, stderr io.Writer) int {
	m := newDebugModel(view)
	m.opts = opts
	m.runEntryIndex = indexOfEntry(view.entryPoints(), opts.Store)
	m.runInput = restoreRunInput(view.path)
	m.runAllStores = restoreRunAll(view.path)
	m.refreshNavFiles()
	progOpts := []tea.ProgramOption{tea.WithOutput(stdout), tea.WithAltScreen()}
	if f, ok := stdin.(*os.File); ok {
		progOpts = append(progOpts, tea.WithInput(stdinNoNamer{f}))
	}
	prog := tea.NewProgram(m, progOpts...)
	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(stderr, "crust develop: %v\n", err)
		return 1
	}
	return 0
}
