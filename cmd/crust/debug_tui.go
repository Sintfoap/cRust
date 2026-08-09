// The bubbletea stepper for `crust develop`. The recording it walks is
// built in internal/debugger (pure Go, no terminal dependency at
// all — that's where the real logic and its test coverage live); this
// file is layout and key handling.
//
// Seven tabs, switched with tab/left/right: Time and Memory (one pie
// chart each, self-time/self-memory per function/loop, plus a few
// overall numbers), Stepper (the recorded tree, navigable with the
// arrow keys — each row shows its source line number, '/' searches by
// label/output text with n/N repeating forward/backward, f/F jump
// straight to the next/previous failed step (all three auto-expanding
// any folded loop lap standing in the way — see jumpToNode), 'v'
// toggles a watch panel showing every variable visible at the selected
// step, read straight from a point-in-time environment snapshot taken
// when that step was traced (see viewStepperWatch), and 'o' toggles a
// panel showing that step's full, untruncated result value (see
// viewStepperDetail — the table's own "out" column is capped at
// maxInspectRunes to stay a fixed width), Editor
// (hands the terminal to nvim on the file being debugged; see
// debug_editor.go for the save-triggered reload loop), Run (type a
// path and press enter to run the file with it as stdin and see the
// raw output, command-line style; see debug_run.go), Bench (run the
// file N times with the Run tab's current settings and chart runtime/
// memory across those runs; see debug_bench.go), and Files (browse and
// switch to another .crust file alongside the one currently open; see
// debug_nav.go).
package main

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
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
	tabBench
	// tabNav is appended last, after every pre-existing tab, so none of
	// their const values (or anything that stores one, like a saved
	// m.active) shift out from under it.
	tabNav
)

// tabCount is how many tabs there are, for wrapping cursor arithmetic —
// named rather than inlined as a literal so handleKey's wraparound math
// stays correct if another tab ever shows up.
const tabCount = 7

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

	// benchCount/benchRuns/benchErr/benchShow back the Bench tab
	// (debug_bench.go): benchCount is the "how many runs" text field
	// (reusing runInputModel, digits only — see handleBenchTabKey),
	// benchRuns is the last batch's raw per-run measurements in run
	// order, benchErr is the last attempt's error (an invalid run count
	// or an unreadable input file — never a per-run program failure,
	// which lives in benchRuns[i].failed instead), and benchShow tracks
	// which of the five toggleable chart lines (raw/average/median/max/
	// min) are currently drawn, shared identically between the Runtime
	// and Memory charts.
	benchCount runInputModel
	benchRuns  []benchRun
	benchErr   string
	benchShow  [benchSeriesKindCount]bool

	// benchInspect/benchCursor back the Bench tab's inspect mode
	// (debug_bench.go's handleBenchTabKey/viewBench), on direct
	// request: a way to navigate inside the charts and see an
	// individual run's exact stats rather than only the aggregate
	// lines. benchInspect is whether 'i' has toggled it on; benchCursor
	// is the currently-selected run's index into benchRuns, valid only
	// while benchInspect is true (and only meaningful once benchRuns is
	// non-empty — toggling on with no runs yet is a no-op).
	benchInspect bool
	benchCursor  int

	// benchExportStatus is the Bench tab's one-line feedback for the
	// last 'e' (export to CSV) attempt — "exported to ..." or "export
	// failed: ...", on direct request ("export Bench results to CSV").
	// Cleared on a fresh batch of runs (handleBenchResult), since it
	// describes the *previous* batch's data, not whatever just ran.
	benchExportStatus string

	// stepSearching/stepQuery/stepSearchInput/stepStatus back the
	// Stepper tab's search ('/') and jump-to-failure (f/F), on direct
	// request: "search/filter the tree" and "jump to next/prev
	// failure." stepSearching is whether the query field currently has
	// focus (typing a new query — handleStepperSearchKey); stepQuery is
	// the last *confirmed* query, kept around so n/N can repeat the same
	// search without retyping it (the same vim `/` then `n`/`N`
	// convention); stepSearchInput is the field itself, reusing
	// runInputModel like every other tab's own text field; stepStatus
	// is transient feedback ("no matches for ...", "no failed steps"),
	// cleared the moment a search or jump actually lands on something.
	stepSearching   bool
	stepQuery       string
	stepSearchInput runInputModel
	stepStatus      string

	// stepWatch backs the Stepper tab's variable watch panel ('v'), on
	// direct request — one of six "what could I add to the develop
	// tool?" ideas, picked to start on and the one expected to matter
	// most day to day. Whether it's toggled on; the panel itself
	// (viewStepperWatch) reads every variable straight from the
	// selected row's own Step.Env, which is already a point-in-time
	// snapshot (object.Environment.Snapshot), so there's no separate
	// state here to keep synchronized with the cursor.
	stepWatch bool

	// stepDetail backs the Stepper tab's full-value detail panel ('o'),
	// on direct request — the last of the same six ideas: shortInspect
	// truncates a long List/Map/String result to maxInspectRunes so the
	// table's own "out" column stays a fixed width, which is exactly
	// right for scanning the tree but means a genuinely large value's
	// tail is simply gone from the row itself. 'o' shows the selected
	// step's untruncated Inspect() text instead, wrapped across the
	// panel's own width rather than the table's fixed column.
	stepDetail bool
}

func newDebugModel(view *debugView) debugModel {
	m := debugModel{view: view, expanded: map[*debugger.TraceNode]bool{}}
	for k := range m.benchShow {
		m.benchShow[k] = true
	}
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
	case benchResultMsg:
		return m.handleBenchResult(msg)
	case benchExportMsg:
		return m.handleBenchExportResult(msg)
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
	// The Bench tab's run-count field and its own toggle mnemonics
	// (r/a/m/x/n) need the same dedicated handling, for the same reason.
	if m.active == tabBench {
		return m.handleBenchTabKey(msg)
	}
	// Same reasoning as the Run tab's own field above: typing a new
	// filename wants nearly every key for itself (including letters
	// that mean something elsewhere, like q or j/k), so it gets its own
	// handler the moment the prompt is up rather than a case threaded
	// through the switch below.
	if m.active == tabNav && m.navCreating {
		return m.handleNavCreateKey(msg)
	}
	// Same reasoning again: a search query can contain letters bound to
	// actions elsewhere (q, f, n, ...), so typing one needs its own
	// handler rather than a case in the switch below.
	if m.active == tabStepper && m.stepSearching {
		return m.handleStepperSearchKey(msg)
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
		} else if m.active == tabStepper {
			m.repeatStepperSearch(true)
		}
	case "N":
		if m.active == tabStepper {
			m.repeatStepperSearch(false)
		}
	case "/":
		if m.active == tabStepper {
			m.stepSearching = true
			m.stepSearchInput = runInputModel{}
			m.stepStatus = ""
		}
	case "f":
		if m.active == tabStepper {
			m.jumpToFailure(true)
		}
	case "F":
		if m.active == tabStepper {
			m.jumpToFailure(false)
		}
	case "v":
		if m.active == tabStepper {
			m.stepWatch = !m.stepWatch
		}
	case "o":
		if m.active == tabStepper {
			m.stepDetail = !m.stepDetail
		}
	}
	return m, nil
}

// handleStepperSearchKey routes keys while the Stepper tab's search
// field has focus (m.stepSearching) — same "own handler, no shared
// switch" reasoning as the Run/Bench tabs' own fields (handleKey's own
// doc comment): a query can contain letters that mean something
// elsewhere (q, f, n, ...), so those must not leak through as commands
// while typing one.
func (m debugModel) handleStepperSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEsc:
		m.stepSearching = false
	case tea.KeyEnter:
		m.stepSearching = false
		m.confirmStepperSearch()
	case tea.KeyLeft:
		m.stepSearchInput.left()
	case tea.KeyRight:
		m.stepSearchInput.right()
	case tea.KeyBackspace:
		m.stepSearchInput.backspace()
	case tea.KeyDelete:
		m.stepSearchInput.deleteForward()
	case tea.KeyHome, tea.KeyCtrlA:
		m.stepSearchInput.cursor = 0
	case tea.KeyEnd, tea.KeyCtrlE:
		m.stepSearchInput.cursor = len(m.stepSearchInput.value)
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			m.stepSearchInput.insert(r)
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
	m.clampAndScrollCursor()
}

// clampAndScrollCursor keeps m.cursor in range and slides m.top (the
// first visible row) to keep it on screen — the scrolling half of
// moveCursor, pulled out on its own so jumpToNode (search, f/F) can
// reuse the exact same "make the target visible" logic after setting
// m.cursor directly, rather than a second copy of the scrolling math
// that could drift out of sync with moveCursor's.
func (m *debugModel) clampAndScrollCursor() {
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

// currentNode is the tree node under the cursor right now — the
// starting point search and jump-to-failure both search outward from.
// nil if there's nothing recorded at all.
func (m debugModel) currentNode() *debugger.TraceNode {
	if len(m.rows) == 0 {
		return nil
	}
	return m.rows[m.cursor].node
}

// flattenAll walks nodes and all their descendants, in recording
// order, regardless of Folded — the order search and jump-to-failure
// both need, since a match can be buried inside a folded loop lap that
// the visible rows (buildRows, which stops at an unexpanded fold)
// currently hide. The underlying tree never mutates after a recording
// finishes (only which folds are expanded does), so this order is
// stable across repeated calls within one session.
func flattenAll(nodes []*debugger.TraceNode) []*debugger.TraceNode {
	var out []*debugger.TraceNode
	for _, n := range nodes {
		out = append(out, n)
		out = append(out, flattenAll(n.Children)...)
	}
	return out
}

// expandPathTo marks every folded ancestor of target (found anywhere
// under nodes) as expanded, so a subsequent rebuildRows makes target's
// own row actually appear — search and jump-to-failure both find
// matches via flattenAll, which sees straight through folds; without
// this, landing the cursor on a match still hidden behind an
// unexpanded fold would be indistinguishable from the jump silently
// failing. Reports whether target was found at all (an internal
// consistency check — false should never actually happen, since every
// candidate target comes from this same tree via flattenAll).
func expandPathTo(nodes []*debugger.TraceNode, target *debugger.TraceNode, expanded map[*debugger.TraceNode]bool) bool {
	for _, n := range nodes {
		if n == target {
			return true
		}
		if expandPathTo(n.Children, target, expanded) {
			expanded[n] = true
			return true
		}
	}
	return false
}

// jumpToNode expands whatever folds are in the way of target, rebuilds
// the visible rows, and scrolls the cursor onto target's own row —
// the shared landing logic search and jump-to-failure both use once
// they've picked a target via findNext.
func (m *debugModel) jumpToNode(target *debugger.TraceNode) {
	expandPathTo(m.view.rec.Roots(), target, m.expanded)
	m.rebuildRows()
	for i, row := range m.rows {
		if row.node == target && !row.closing {
			m.cursor = i
			break
		}
	}
	m.clampAndScrollCursor()
}

// findNext returns the first node in all (flattenAll's recording
// order) after start for which match is true, wrapping around the end
// (forward) or the beginning (backward) if nothing matches before
// reaching it again — nil only if nothing in all matches at all. start
// == nil (nothing selected yet) searches the whole list starting from
// the first (forward) or last (backward) node.
func findNext(all []*debugger.TraceNode, start *debugger.TraceNode, forward bool, match func(*debugger.TraceNode) bool) *debugger.TraceNode {
	n := len(all)
	if n == 0 {
		return nil
	}
	startIdx := -1
	if start != nil {
		for i, node := range all {
			if node == start {
				startIdx = i
				break
			}
		}
	}
	for step := 1; step <= n; step++ {
		var i int
		if forward {
			i = (startIdx + step) % n
		} else {
			i = ((startIdx-step)%n + n) % n
		}
		if match(all[i]) {
			return all[i]
		}
	}
	return nil
}

// isFailedStep is jumpToFailure's match predicate: a real step (not a
// frame) whose result was a runtime Error.
func isFailedStep(n *debugger.TraceNode) bool {
	return !n.IsFrame() && n.Step.Failed()
}

// jumpToFailure moves the cursor to the next (or, forward == false,
// previous) failed step from wherever it currently is, wrapping around
// the whole recording — on direct request, "jump to next/prev
// failure." Leaves the cursor untouched and reports it in stepStatus
// when there are no failed steps at all, rather than silently doing
// nothing.
func (m *debugModel) jumpToFailure(forward bool) {
	target := findNext(flattenAll(m.view.rec.Roots()), m.currentNode(), forward, isFailedStep)
	if target == nil {
		m.stepStatus = "no failed steps"
		return
	}
	m.stepStatus = ""
	m.jumpToNode(target)
}

// stepMatches reports whether n's own rendered text — its label, plus
// a step's own output — contains query, case-insensitively. Searching
// exactly the text already visible in the table (renderRow) means a
// match found by '/' is always recognizable once the cursor lands on
// it, with no separate "what did it actually match on" question.
func stepMatches(n *debugger.TraceNode, query string) bool {
	text := n.Label()
	if !n.IsFrame() {
		text += " " + shortInspect(n.Step.Out)
	}
	return strings.Contains(strings.ToLower(text), strings.ToLower(query))
}

// confirmStepperSearch runs when '/' search input is confirmed with
// Enter: an empty query just closes the field with no search
// performed (matching how pressing enter on an empty vim `/` prompt
// does nothing), otherwise it becomes the new m.stepQuery (so n/N can
// repeat it) and the cursor jumps to the first match from its current
// position.
func (m *debugModel) confirmStepperSearch() {
	query := strings.TrimSpace(m.stepSearchInput.String())
	if query == "" {
		return
	}
	m.stepQuery = query
	m.jumpToNextMatch(true)
}

// repeatStepperSearch is 'n'/'N': re-run the last confirmed search
// (m.stepQuery) forward or backward from the cursor's current
// position, without needing to retype it — the same vim `/` then n/N
// convention. A no-op if nothing's ever been searched for yet.
func (m *debugModel) repeatStepperSearch(forward bool) {
	if m.stepQuery == "" {
		return
	}
	m.jumpToNextMatch(forward)
}

func (m *debugModel) jumpToNextMatch(forward bool) {
	query := m.stepQuery
	target := findNext(flattenAll(m.view.rec.Roots()), m.currentNode(), forward, func(n *debugger.TraceNode) bool {
		return stepMatches(n, query)
	})
	if target == nil {
		m.stepStatus = fmt.Sprintf("no matches for %q", query)
		return
	}
	m.stepStatus = ""
	m.jumpToNode(target)
}

// maxWatchLines caps the variable watch panel's own height — enough
// lines to reserve a fixed budget for it in stepperExtraLines
// regardless of how many variables happen to be visible at whichever
// row is selected (a deeply nested recipe call could have dozens),
// rather than a reservation that changes shape every time the cursor
// moves and could fight with stepperBodyHeight's own row count.
const maxWatchLines = 8

// maxDetailLines caps the full-value detail panel's own height, the
// same fixed-worst-case reservation shape maxWatchLines already
// established — a huge List/Map's Inspect() text could wrap across
// far more lines than any reasonable panel should claim.
const maxDetailLines = 10

// stepperExtraLines is how many lines viewStepper prints beyond the
// fixed baseline (the tree table itself) — the search field or a
// confirmed query reminder above the header, a status message, and/or
// the watch/detail panels below the table — computed here rather than
// duplicated as a second copy of the same conditions inside
// viewStepper, so stepperBodyHeight's reservation can never drift out
// of sync with what actually gets rendered.
func (m debugModel) stepperExtraLines() int {
	extra := 0
	if m.stepSearching || m.stepQuery != "" {
		extra += 2
	}
	if m.stepStatus != "" {
		extra += 2
	}
	if m.stepWatch {
		extra += maxWatchLines + 2
	}
	if m.stepDetail {
		extra += maxDetailLines + 2
	}
	return extra
}

// stepperBodyHeight is how many tree rows fit on screen at once, below
// the tab bar, the column header, and above the help footer — and
// below the search/status lines (stepperExtraLines), when showing.
func (m debugModel) stepperBodyHeight() int {
	h := m.height - 6 - m.stepperExtraLines()
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
	case tabBench:
		body = m.viewBench()
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
		if m.stepSearching {
			return "enter: search   esc: cancel   ctrl+c: quit"
		}
		return "tab/←→: switch tab   ↑↓: move   enter: open/close   /: search   n/N: next/prev match   f/F: next/prev failure   v: watch variables   o: full value   q: quit"
	case tabEditor:
		return "enter: reopen nvim   tab/←→: switch tab   q: quit"
	case tabRun:
		if len(m.runEntryOptions()) > 0 {
			return "↑↓: field/entry point   ←→: move/change   enter: run   tab/⇧tab: switch tab   ctrl+c/esc: quit"
		}
		return "enter: run   tab/⇧tab: switch tab   ctrl+c/esc: quit"
	case tabBench:
		if m.benchInspect {
			return "h/l: prev/next run   g/G: first/last run   i: exit inspect   e: export CSV   r/a/m/x/n: toggle a line   tab/⇧tab: switch tab   ctrl+c/esc: quit"
		}
		return "enter: run N times   i: inspect a run   e: export CSV   r/a/m/x/n: toggle a line   tab/⇧tab: switch tab   ctrl+c/esc: quit"
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
	timeS, memS, stepper, editor, run, bench, nav := styleTabIdle, styleTabIdle, styleTabIdle, styleTabIdle, styleTabIdle, styleTabIdle, styleTabIdle
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
	case tabBench:
		bench = styleTabActive
	case tabNav:
		nav = styleTabActive
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		timeS.Render("Time"), memS.Render("Memory"), stepper.Render("Stepper"),
		editor.Render("Editor"), run.Render("Run"), bench.Render("Bench"), nav.Render("Files"))
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
	chart := pieChart(kpis, m.pieRadius(len(kpis)), func(k debugger.KPI) float64 { return float64(k.SelfTime) },
		func(k debugger.KPI) string { return k.SelfTime.String() })
	col := lipgloss.JoinVertical(lipgloss.Left, styleTitle.Render("time by function"), chart)
	return col + "\n\n" + m.viewTimeStats()
}

// pieRadius picks the Time/Memory tabs' pie chart radius from the
// window's actual size, on direct request ("adaptive pie chart radius
// that adapts to the terminal's actual size," a stretch item this
// closes out): fixed at 7 before this, the chart's own height (2r+1
// rows) plus a growing legend (one line per KPI family) could run past
// the bottom of a short terminal, and clampHeight — built for exactly
// this failure mode elsewhere — can only trim the *result*, not shrink
// the chart itself back into the space available. reserved bundles
// every other line View()'s frame is already committed to printing
// around the chart (tab bar, header, chart title, the blank line
// pieChart itself leaves between the circle and its legend, the blank
// line before the stats block, up to three stats lines — viewTimeStats
// is the longer of the two callers' stats blocks, so both tabs share
// one conservative reservation rather than computing two slightly
// different ones — and the help footer): the same "how much is already
// spoken for" bookkeeping stepperBodyHeight/benchChartHeight already
// do for their own tabs. Never grows past 7 — nothing asked for a
// *bigger* chart on a roomy terminal, only for a small one to stop
// losing its bottom rows — and never shrinks below 3, since a smaller
// circle stops reading as one at all. Also bounded by width (each unit
// of radius costs 4 more character columns), for the rare narrow-but-
// tall terminal.
func (m debugModel) pieRadius(kpiCount int) int {
	const reserved = 10
	byHeight := (m.height - reserved - kpiCount - 1) / 2
	byWidth := (m.width - 2) / 4
	radius := min(byHeight, byWidth, 7)
	return max(radius, 3)
}

// viewMemory is viewTime's counterpart for self-size — same chart
// function, same overall-numbers idea, but sized/labeled for the
// question "where did the memory go" instead of "where did the time
// go" (largest single value in place of slowest statement, since
// duration doesn't apply to a memory reading).
func (m debugModel) viewMemory() string {
	kpis := m.view.timing().KPIs()
	chart := pieChart(kpis, m.pieRadius(len(kpis)), func(k debugger.KPI) float64 { return float64(k.SelfSize) },
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
// Above the column header: the search field while typing one ('/'),
// or a reminder of the last confirmed query once it's closed (n/N
// still work off it), and/or a status line ("no matches for ...", "no
// failed steps") — stepperExtraLines' own doc comment on why the
// exact same condition also drives stepperBodyHeight's reservation.
func (m debugModel) viewStepper() string {
	if len(m.rows) == 0 {
		return styleMuted.Render("(nothing recorded)")
	}
	t := m.view.timing()

	var b strings.Builder
	switch {
	case m.stepSearching:
		b.WriteString(styleTitle.Render("/"))
		b.WriteString(m.stepSearchInput.render())
		b.WriteString("\n\n")
	case m.stepQuery != "":
		b.WriteString(styleMuted.Render(fmt.Sprintf("search: %q (n/N: next/prev match)", m.stepQuery)))
		b.WriteString("\n\n")
	}
	if m.stepStatus != "" {
		b.WriteString(styleError.Render(m.stepStatus))
		b.WriteString("\n\n")
	}

	b.WriteString(styleFaint.Render(fmt.Sprintf("%5s %s %s %6s %9s %7s\n",
		"line", col("step", 40), col("out", 18), "size", "time", "self%")))

	visible := m.stepperBodyHeight()
	end := m.top + visible
	if end > len(m.rows) {
		end = len(m.rows)
	}
	for i := m.top; i < end; i++ {
		b.WriteString(m.renderRow(i, t))
		b.WriteByte('\n')
	}
	if m.stepWatch {
		b.WriteByte('\n')
		b.WriteString(m.viewStepperWatch())
	}
	if m.stepDetail {
		b.WriteByte('\n')
		b.WriteString(m.viewStepperDetail())
	}
	return b.String()
}

// viewStepperWatch is the variable watch panel ('v'), on direct
// request — one of six "what could I add to the develop tool?" ideas.
// A step's own "out" column only ever shows what that one statement
// itself evaluated to; watch mode answers the more common debugging
// question, "what was every variable at this point," straight from
// Step.Env — a snapshot object.Environment.Snapshot already took at
// trace time, so this never re-reads live (and by now long-since-
// mutated) interpreter state.
//
// A frame or closing row has no Step of its own to read variables
// from — a loop lap or recipe call's *entry* state isn't captured
// anywhere, only each statement inside it — so the panel says so
// rather than silently showing nothing or the wrong row's data.
func (m debugModel) viewStepperWatch() string {
	row := m.rows[m.cursor]
	if row.closing || row.node.IsFrame() {
		return styleMuted.Render("(select a step, not a frame, to watch its variables)")
	}
	env := row.node.Step.Env
	if len(env) == 0 {
		return styleMuted.Render("(no variables visible at this step)")
	}

	names := make([]string, 0, len(env))
	for name := range env {
		names = append(names, name)
	}
	slices.Sort(names)

	shown := names
	truncated := 0
	if len(shown) > maxWatchLines {
		truncated = len(shown) - maxWatchLines
		shown = shown[:maxWatchLines]
	}

	var b strings.Builder
	b.WriteString(styleTitle.Render(fmt.Sprintf("variables at this step (%d)", len(names))))
	b.WriteByte('\n')
	for _, name := range shown {
		fmt.Fprintf(&b, "%s = %s\n", styleMuted.Render(name), shortInspect(env[name]))
	}
	if truncated > 0 {
		b.WriteString(styleFaint.Render(fmt.Sprintf("… and %d more\n", truncated)))
	}
	return b.String()
}

// viewStepperDetail is the full-value detail panel ('o'), on direct
// request — the last of the same six ideas: the table's own "out"
// column runs every value through shortInspect, truncated at
// maxInspectRunes to keep the column a fixed width, which reads fine
// for scanning the tree but throws away the tail of anything genuinely
// large. This panel shows the selected step's *untruncated* Inspect()
// text instead, hard-wrapped across the panel's own width (wrapRunes)
// rather than the table's fixed column, capped at maxDetailLines the
// same way viewStepperWatch caps its own row count.
//
// Same frame/closing-row guard as the watch panel, for the same
// reason: there's no Step to read Out from on a row that isn't one.
func (m debugModel) viewStepperDetail() string {
	row := m.rows[m.cursor]
	if row.closing || row.node.IsFrame() {
		return styleMuted.Render("(select a step, not a frame, to see its full value)")
	}
	text := "nobox"
	if row.node.Step.Out != nil {
		text = row.node.Step.Out.Inspect()
	}

	lines := wrapRunes(text, m.stepperDetailWidth())
	shown := lines
	truncated := 0
	if len(shown) > maxDetailLines {
		truncated = len(shown) - maxDetailLines
		shown = shown[:maxDetailLines]
	}

	var b strings.Builder
	b.WriteString(styleTitle.Render(fmt.Sprintf("full value (%d chars)", len([]rune(text)))))
	b.WriteByte('\n')
	for _, l := range shown {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	if truncated > 0 {
		b.WriteString(styleFaint.Render(fmt.Sprintf("… (%d more line(s) — grow the terminal to see them)\n", truncated)))
	}
	return b.String()
}

// stepperDetailWidth is how wide viewStepperDetail wraps its text —
// the window's own width, minus a small margin, the same "derive from
// m.width, clamp to a sane floor" shape benchChartWidth already uses.
func (m debugModel) stepperDetailWidth() int {
	return max(20, m.width-4)
}

// wrapRunes hard-wraps s into width-rune chunks — no word-boundary
// smartness, since a raw Inspect() dump (`[1, 2, 3, ...]`) has no
// natural word breaks worth preserving anyway. width <= 0 (a
// pathologically narrow terminal) returns s as a single unwrapped
// line rather than looping forever.
func wrapRunes(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	runes := []rune(s)
	if len(runes) == 0 {
		return []string{""}
	}
	var lines []string
	for len(runes) > width {
		lines = append(lines, string(runes[:width]))
		runes = runes[width:]
	}
	lines = append(lines, string(runes))
	return lines
}

// stepLineText is a step's source line number, right-aligned to match
// the header's "line" column — blank for a frame row (a recipe call or
// loop lap has no single line of its own) or a step whose Line() came
// back 0 (Pos unset — shouldn't happen for parser-produced AST, but a
// blank beats a misleading "0").
func stepLineText(n *debugger.TraceNode) string {
	if n.IsFrame() {
		return ""
	}
	if line := n.Step.Line(); line > 0 {
		return strconv.Itoa(line)
	}
	return ""
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
	line := fmt.Sprintf("%5s %s %s %6s %9s %7s",
		stepLineText(n), col(label, 40), col(out, 18), size, nt.Total.String(), selfPctText(nt, t.Overall()))

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
	line := fmt.Sprintf("%5s %s", "", strings.Repeat("  ", row.depth)+"// end "+closingLabel(row.node.Label()))
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
	m.benchCount = benchCountInput(restoreBenchCount(view.path))
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
