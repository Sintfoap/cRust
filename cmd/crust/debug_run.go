// The Run tab (debug_tui.go's tabRun): type a path to an input file
// and press enter to run the file being debugged with it as stdin,
// showing the raw output — exactly what `crust run file.crust <
// input.txt` would print, not the structured per-statement trace the
// KPI/Stepper tabs show. It reuses runFile (run.go) directly rather
// than going through debugger.Recorder at all for that output, so
// what's shown here is genuinely "the command line," with no tracing
// overhead or KPI bucketing in the way.
//
// Running here also re-records the same entry point + input file
// through debugger.Recorder in the background and swaps it in as the
// Time/Memory/Stepper tabs' recording (handleRunResult) — so whichever
// store/store_<name> (and whichever input file) you just ran with the
// Run tab's own selector is what the rest of the TUI shows too,
// instead of staying pinned to whatever `--store` the session started
// with. That happens on "run" specifically, not the moment the
// selector is cycled, since running is also the only point an input
// file actually gets read — retracing on every arrow-key tap would
// either mean re-reading a possibly-large file on every keystroke, or
// tracing against no real input at all, neither of which is "what
// happened when I ran it."
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Sintfoap/cRust/internal/debugger"
)

// runInputModel is a minimal single-line text field for the input-file
// path — hand-rolled rather than reaching for a components library
// (e.g. charmbracelet/bubbles' textinput): inserting/deleting runes
// around a cursor is a small enough job that a third TUI dependency,
// after bubbletea + lipgloss, isn't worth it under this project's
// add-a-dependency-only-when-needed policy.
type runInputModel struct {
	value  []rune
	cursor int
}

func (in *runInputModel) insert(r rune) {
	in.value = append(in.value[:in.cursor:in.cursor], append([]rune{r}, in.value[in.cursor:]...)...)
	in.cursor++
}

func (in *runInputModel) backspace() {
	if in.cursor == 0 {
		return
	}
	in.value = append(in.value[:in.cursor-1], in.value[in.cursor:]...)
	in.cursor--
}

func (in *runInputModel) deleteForward() {
	if in.cursor >= len(in.value) {
		return
	}
	in.value = append(in.value[:in.cursor], in.value[in.cursor+1:]...)
}

func (in *runInputModel) left() {
	if in.cursor > 0 {
		in.cursor--
	}
}

func (in *runInputModel) right() {
	if in.cursor < len(in.value) {
		in.cursor++
	}
}

func (in runInputModel) String() string {
	return string(in.value)
}

// render draws the field with a reverse-video block standing in for a
// terminal cursor at the current position — a plain lipgloss.Reverse
// on one rune (or a trailing space, past the last rune) reads as a
// cursor regardless of the surrounding theme.
func (in runInputModel) render() string {
	if in.cursor >= len(in.value) {
		return string(in.value) + styleCursor.Render(" ")
	}
	before := string(in.value[:in.cursor])
	at := styleCursor.Render(string(in.value[in.cursor]))
	after := string(in.value[in.cursor+1:])
	return before + at + after
}

// handleRunTabKey routes keys on the Run tab. Which row has focus
// (m.runEntryFocused) decides where most keys go: Left/Right and typed
// characters mean "edit the input-file field" when it has focus, or
// "cycle the entry-point selector" when that row does instead. Almost
// everything typed while the field has focus goes straight to it,
// including letters bound to actions on every other tab (q for quit,
// h/j/k/l for movement), since a file path can legitimately contain
// any of them. Only the handful of keys a path could never need stay
// reserved: Up/Down to move focus between the two rows, Tab/Shift+Tab
// to switch tabs, Enter to run, Ctrl+R to toggle "run all stores"
// (m.runAllStores — see its doc comment on debugModel), Ctrl+F/Ctrl+S/
// Ctrl+L for the manual AoC actions (debug_aoc_actions.go's own doc
// comment explains why Ctrl+<letter> combos specifically are safe
// here), and Ctrl+C/Esc as an always-available way out that doesn't
// depend on typing a bare "q".
func (m debugModel) handleRunTabKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyTab:
		m.active = (m.active + 1) % tabCount
		m.maybeRefreshNav()
		return m, m.maybeOpenEditor()
	case tea.KeyShiftTab:
		m.active = (m.active + tabCount - 1) % tabCount
		m.maybeRefreshNav()
		return m, m.maybeOpenEditor()
	case tea.KeyEnter:
		if m.runAllStores {
			return m, m.runAllStoresCmd()
		}
		return m, m.runProgramCmd()
	case tea.KeyCtrlR:
		m.runAllStores = !m.runAllStores
	case tea.KeyCtrlC, tea.KeyEsc:
		return m, tea.Quit
	case tea.KeyUp, tea.KeyDown:
		m.toggleRunFocus()
	case tea.KeyPgUp:
		m.moveRunOutputScroll(-m.runOutputBodyHeight())
	case tea.KeyPgDown:
		m.moveRunOutputScroll(m.runOutputBodyHeight())
	case tea.KeyCtrlF:
		return m, m.aocFetchCmd()
	case tea.KeyCtrlS:
		return m, m.aocSubmitCmd()
	case tea.KeyCtrlL:
		return m, loginCmd()
	case tea.KeyLeft:
		if m.runEntryFocused {
			m.cycleRunEntry(-1)
		} else {
			m.runInput.left()
		}
	case tea.KeyRight:
		if m.runEntryFocused {
			m.cycleRunEntry(1)
		} else {
			m.runInput.right()
		}
	case tea.KeyRunes:
		if !m.runEntryFocused {
			for _, r := range msg.Runes {
				m.runInput.insert(r)
			}
		}
	case tea.KeySpace:
		if !m.runEntryFocused {
			m.runInput.insert(' ')
		}
	case tea.KeyBackspace:
		if !m.runEntryFocused {
			m.runInput.backspace()
		}
	case tea.KeyDelete:
		if !m.runEntryFocused {
			m.runInput.deleteForward()
		}
	case tea.KeyHome, tea.KeyCtrlA:
		if !m.runEntryFocused {
			m.runInput.cursor = 0
		}
	case tea.KeyEnd, tea.KeyCtrlE:
		if !m.runEntryFocused {
			m.runInput.cursor = len(m.runInput.value)
		}
	}
	return m, nil
}

// toggleRunFocus switches between the input-file field and the
// entry-point selector, when there's actually a selector to switch
// to — a file with no store/store_<name> recipes at all (the common
// case for a small top-to-bottom AoC script) has nothing to select, so
// Up/Down stays a no-op and focus stays on the field.
func (m *debugModel) toggleRunFocus() {
	if len(m.runEntryOptions()) == 0 {
		return
	}
	m.runEntryFocused = !m.runEntryFocused
}

// cycleRunEntry moves the selected entry point by delta, wrapping —
// a small, fixed-size row of choices, so wraparound (like the tab
// bar's own) reads better than clamping at the ends.
func (m *debugModel) cycleRunEntry(delta int) {
	n := len(m.runEntryOptions())
	if n == 0 {
		return
	}
	m.runEntryIndex = ((m.runEntryIndex+delta)%n + n) % n
}

// runEntryOptions returns the file's store/store_<name> entry points
// (debugView.entryPoints, scanned once and cached there per view), for
// the Run tab's selector.
func (m debugModel) runEntryOptions() []string {
	return m.view.entryPoints()
}

// selectedRunEntry returns the currently selected entry point's
// --store value ("" for the bare `store`), or "" when the file has no
// entry points to select from — runFile already treats an empty store
// the same way `crust run` with no --store flag does.
func (m debugModel) selectedRunEntry() string {
	options := m.runEntryOptions()
	if len(options) == 0 {
		return ""
	}
	i := m.runEntryIndex
	if i < 0 || i >= len(options) {
		i = 0
	}
	return options[i]
}

// indexOfEntry returns store's position among options ("" for the bare
// `store`, same convention collectEntryPoints uses), or 0 if store
// isn't among them (including when options is empty) — 0 is always a
// vacuous-but-safe default, since runEntryOptions()/selectedRunEntry()
// only ever consult it when len(options) > 0.
func indexOfEntry(options []string, store string) int {
	for i, o := range options {
		if o == store {
			return i
		}
	}
	return 0
}

// runResultMsg carries one Run-tab execution's result: the raw
// command-line-style output (stdout/stderr/code/err, as before), plus
// a freshly retraced view of that same run for the Time/Memory/
// Stepper tabs. view is nil if the retrace itself couldn't run (e.g.
// the input file vanished between the two reads) — handleRunResult
// leaves the previous recording in place rather than blanking those
// tabs out over what the Run tab still managed to execute and show.
type runResultMsg struct {
	stdout string
	stderr string
	code   int
	err    error // couldn't open the given input file; distinct from a program-level failure, which shows up in stderr/code instead

	view  *debugView // retraced with the same store + input; nil if the retrace failed
	store string     // the store the retrace used, so handleRunResult can make it "current"
}

// runProgramCmd runs m.view.path exactly the way `crust run` would —
// same runFile (run.go), with whichever entry point the Run tab's
// selector currently has picked — with the Run tab's input-file path
// (if any) read once and fed as stdin to two independent runs: the
// untraced one whose raw output lands on the Run tab (unchanged from
// before), and a second, traced one (via buildDebugView, same as the
// Editor tab's save-triggered reload) whose recording becomes the new
// Time/Memory/Stepper data. Reading the input file once and reusing
// its bytes for both, rather than opening it twice, means the two runs
// can't ever see different content even if the file changes between
// them, and doubles as the "no input file selected" case for free:
// inputPath == "" leaves data nil, and bytes.NewReader(nil) reads as
// immediate EOF exactly like the old strings.NewReader("") did.
func (m debugModel) runProgramCmd() tea.Cmd {
	path, store := m.view.path, m.selectedRunEntry()
	inputPath := strings.TrimSpace(m.runInput.String())
	maxSteps := m.opts.MaxSteps
	return func() tea.Msg {
		var data []byte
		if inputPath != "" {
			d, err := os.ReadFile(inputPath)
			if err != nil {
				return runResultMsg{err: err}
			}
			data = d
		}

		// Persisted now, not only after a successful trace below --
		// what matters for "remember my last choice" is that the input
		// file was actually readable and a run was attempted, not
		// whether the crust program itself then ran cleanly or the
		// retrace happened to succeed.
		saveDevelStateBestEffort(path, store, inputPath, false)

		var stdout, stderr bytes.Buffer
		code := runFile(path, store, bytes.NewReader(data), &stdout, &stderr)

		traceOpts := debugOptions{Store: store, MaxSteps: maxSteps}
		view, _ := buildDebugView(path, traceOpts, bytes.NewReader(data), io.Discard)

		return runResultMsg{
			stdout: stdout.String(), stderr: stderr.String(), code: code,
			view: view, store: store,
		}
	}
}

// entryLabel renders store for a human-readable heading — "(default)"
// for the bare `store`, the same convention renderEntryOptions already
// uses for the selector row itself, so runAllStoresCmd's per-store
// output headings read consistently with it.
func entryLabel(store string) string {
	if store == "" {
		return "(default)"
	}
	return store
}

// entryFrameLabel renders store as the wrapper frame runAllStoresCmd
// merges one store's recording under — the same target+"(...)" shape
// interpreter.CallNamed itself already labels a traced entry-point
// call with (SPEC.md §9's store/store_<name>), so a merged run's
// Stepper/KPI rows read exactly like an ordinary single-store one
// would, just one level further out.
func entryFrameLabel(store string) string {
	target := "store"
	if store != "" {
		target = "store_" + store
	}
	return target + "(...)"
}

// runAllStoresCmd runs every entry point m.runEntryOptions() reports
// against the Run tab's input file, one complete run per store, in
// sequence — falling back to a single bare-`store` run when the file
// declares no store/store_<name> entry points at all, so toggling
// "run all" on for a plain top-to-bottom script is never a dead end.
// Each store gets its own fresh runFile/buildDebugView call (its own
// Interpreter, environment, and bytes.NewReader(data) over the input),
// deliberately not one shared Interpreter reused across the loop:
// unbox() with no argument reads whichever stdin was bound at
// construction (internal/builtins.New's doc comment), so sharing one
// would leave every store after the first reading an already-drained
// reader instead of the fresh, full view of "the same input file" this
// feature promises — exactly what separately typing `crust day01.crust
// --store=part1 < input.txt` and `crust day01.crust --store=part2 <
// input.txt` at a real shell would each get, which is the behavior
// this reproduces.
//
// The raw output concatenates every store's stdout+stderr under a
// "=== <store> ===" heading, in entry-point order — what the Run tab
// shows. The traced recordings combine into one debugger.Recorder via
// Merge, one wrapper frame per store (entryFrameLabel), so the Time/
// Memory/Stepper tabs show every store's timing/KPI data together
// rather than only whichever ran last.
func (m debugModel) runAllStoresCmd() tea.Cmd {
	path := m.view.path
	options := m.runEntryOptions()
	if len(options) == 0 {
		options = []string{""}
	}
	inputPath := strings.TrimSpace(m.runInput.String())
	maxSteps := m.opts.MaxSteps
	selected := m.selectedRunEntry()

	return func() tea.Msg {
		var data []byte
		if inputPath != "" {
			d, err := os.ReadFile(inputPath)
			if err != nil {
				return runResultMsg{err: err}
			}
			data = d
		}

		// Same "persist regardless of how the run itself turns out"
		// reasoning as runProgramCmd above -- selected (the Run tab's
		// own selector position) is what's remembered as "the" store,
		// not any one store from the loop below, matching what stays
		// highlighted in the selector while "run all" is on.
		saveDevelStateBestEffort(path, selected, inputPath, true)

		var out strings.Builder
		failed := false
		merged := debugger.NewRecorder(maxSteps)
		for i, store := range options {
			if i > 0 {
				out.WriteByte('\n')
			}
			fmt.Fprintf(&out, "=== %s ===\n", entryLabel(store))

			var stdout, stderr bytes.Buffer
			code := runFile(path, store, bytes.NewReader(data), &stdout, &stderr)
			if code != 0 {
				failed = true
			}

			storeOut := stdout.String()
			if stderr.Len() > 0 {
				if storeOut != "" && !strings.HasSuffix(storeOut, "\n") {
					storeOut += "\n"
				}
				storeOut += stderr.String()
			}
			if storeOut == "" {
				storeOut = "(no output)\n"
			} else if !strings.HasSuffix(storeOut, "\n") {
				storeOut += "\n"
			}
			out.WriteString(storeOut)

			traceOpts := debugOptions{Store: store, MaxSteps: maxSteps}
			if view, err := buildDebugView(path, traceOpts, bytes.NewReader(data), io.Discard); err == nil {
				merged.Merge(entryFrameLabel(store), view.rec)
			}
		}

		code := 0
		if failed {
			code = 1
		}
		return runResultMsg{
			stdout: out.String(), code: code,
			view: &debugView{path: path, rec: merged}, store: selected,
		}
	}
}

// handleRunResult applies one Run-tab execution's result to the model.
// stdout and stderr are shown concatenated, in that order, matching
// what a real terminal running the same command would show as long as
// the program doesn't interleave them faster than its own buffering —
// good enough for "what did this do", which is the question this tab
// answers.
//
// When the accompanying retrace succeeded (msg.view != nil), it also
// replaces the Time/Memory/Stepper tabs' recording and resets the
// Stepper's fold/cursor state, the same way a successful Editor-tab
// reload does (handleReload) — the new tree has no relationship to the
// old one's. m.opts.Store is updated too, so this selection becomes
// "current" for the rest of the session: a later Editor-tab save
// reloads with this same store rather than reverting to whatever
// --store the session originally started with. Staying on the Run tab
// (not switching to Time, unlike a reload) is deliberate — the user is
// still mid-interaction here, picking entry points and input files,
// not asking to be taken to the dashboard.
func (m debugModel) handleRunResult(msg runResultMsg) (tea.Model, tea.Cmd) {
	m.runOutputTop = 0
	if msg.err != nil {
		m.runOutput = msg.err.Error()
		m.runFailed = true
		return m, nil
	}
	out := msg.stdout
	if msg.stderr != "" {
		if out != "" && !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		out += msg.stderr
	}
	if out == "" {
		out = "(no output)"
	}
	m.runOutput = out
	m.runFailed = msg.code != 0

	if msg.view != nil {
		m.view = msg.view
		m.opts.Store = msg.store
		m.expanded = map[*debugger.TraceNode]bool{}
		m.cursor, m.top = 0, 0
		m.rebuildRows()
	}
	return m, nil
}

// viewRun is the Run tab: the input-file field, the entry-point
// selector (only when the file actually has store/store_<name> recipes
// to choose from), then either instructions (nothing has run yet) or
// the last run's output. A "> " marker in front of whichever row has
// focus mirrors the Stepper's own cursor-row convention, rather than
// inventing a second way to show "this is the active one."
func (m debugModel) viewRun() string {
	var b strings.Builder

	inputMarker := "  "
	if !m.runEntryFocused {
		inputMarker = "> "
	}
	b.WriteString(inputMarker + styleTitle.Render("input file (optional, becomes stdin):"))
	b.WriteByte('\n')
	b.WriteString("  " + m.runInput.render())
	b.WriteString("\n\n")

	if options := m.runEntryOptions(); len(options) > 0 {
		entryMarker := "  "
		if m.runEntryFocused {
			entryMarker = "> "
		}
		b.WriteString(entryMarker + styleTitle.Render("entry point (↑↓ to select this row, ←→ to change):"))
		b.WriteByte('\n')
		b.WriteString("  " + renderEntryOptions(options, m.runEntryIndex))
		b.WriteString("\n\n")

		runAllText := "ctrl+r: run all stores in sequence, off"
		if m.runAllStores {
			runAllText = "ctrl+r: run all stores in sequence, ON — every entry point above runs against the same input, one after another"
		}
		b.WriteString("  " + styleMuted.Render(runAllText))
		b.WriteString("\n\n")
	}

	if m.aocActionStatus != "" {
		b.WriteString("  " + styleMuted.Render(m.aocActionStatus))
		b.WriteString("\n\n")
	}

	if m.runOutput == "" {
		instructions := fmt.Sprintf(
			"enter: run %s (with the path above as stdin, if any) and show its output here, like running it from the command line — also updates the Time/Memory/Stepper tabs to match this run",
			m.view.path)
		if m.runAllStores {
			instructions = fmt.Sprintf(
				"enter: run every entry point of %s in sequence (with the path above as stdin for each) and show all their output here — also merges all their timing/debug info into the Time/Memory/Stepper tabs",
				m.view.path)
		}
		b.WriteString(styleMuted.Render(instructions))
	} else {
		style := lipgloss.NewStyle()
		if m.runFailed {
			style = styleError
		}
		b.WriteString(style.Render(m.scrolledRunOutput()))
	}
	return b.String()
}

// runOutputExtraLines is how many lines viewRun prints before the
// output panel itself — the input-file field's own 3 lines (label,
// field, blank) plus, only when the file actually has entry points to
// choose from, the entry-point selector's own 5 (label, options row,
// blank, run-all line, blank), plus 2 more whenever ctrl+f/ctrl+s/
// ctrl+l last left a status message up (the line itself, plus the
// blank line after it). The same "compute the fixed chrome budget once
// so the body-height calculation can't drift out of sync with what
// actually renders" reasoning stepperExtraLines/liveExtraLines already
// established for their own tabs.
func (m debugModel) runOutputExtraLines() int {
	extra := 3
	if len(m.runEntryOptions()) > 0 {
		extra += 5
	}
	if m.aocActionStatus != "" {
		extra += 2
	}
	return extra
}

// runOutputBodyHeight is how many lines of runOutput fit on screen at
// once — liveBodyHeight/stepperBodyHeight's own budget shape, applied
// to the output panel. -2 is reserved unconditionally for the
// scrolled-view's own "N more above"/"N more below" hint lines
// (scrolledRunOutput) — the same worst-case-always-reserved trade-off
// liveExtraLines makes for the Live tab's watch panel: simpler than
// solving the fixed point of "does reserving space for hints change
// whether hints are needed," at the cost of up to 2 lines of otherwise
// visible output on the rare run that lands exactly on that boundary.
func (m debugModel) runOutputBodyHeight() int {
	h := m.height - 6 - m.runOutputExtraLines() - 2
	if h < 3 {
		h = 3
	}
	return h
}

// scrolledRunOutput returns the runOutputTop-scrolled window into
// runOutput's own lines — the whole thing unchanged (no hints) when it
// already fits, on direct request ("some sort of scrollable-ness on
// the output") for output too tall to fit the terminal at once, the
// same problem the Live tab's watch panel had before liveWatchTop.
func (m debugModel) scrolledRunOutput() string {
	lines := strings.Split(m.runOutput, "\n")
	visible := m.runOutputBodyHeight()
	if len(lines) <= visible {
		return m.runOutput
	}

	top := m.runOutputTop
	maxTop := len(lines) - visible
	if top > maxTop {
		top = maxTop
	}
	if top < 0 {
		top = 0
	}
	end := top + visible
	if end > len(lines) {
		end = len(lines)
	}

	var b strings.Builder
	if top > 0 {
		b.WriteString(styleFaint.Render(fmt.Sprintf("… %d more line(s) above (pgup)", top)))
		b.WriteByte('\n')
	}
	b.WriteString(strings.Join(lines[top:end], "\n"))
	if end < len(lines) {
		b.WriteByte('\n')
		b.WriteString(styleFaint.Render(fmt.Sprintf("… %d more line(s) below (pgdn)", len(lines)-end)))
	}
	return b.String()
}

// moveRunOutputScroll scrolls runOutputTop by delta lines, clamped the
// same way moveLiveWatchCursor clamps liveWatchTop — never above 0,
// never past the point where the last line would leave the window
// early.
func (m *debugModel) moveRunOutputScroll(delta int) {
	lines := strings.Split(m.runOutput, "\n")
	maxTop := len(lines) - m.runOutputBodyHeight()
	if maxTop < 0 {
		maxTop = 0
	}
	m.runOutputTop += delta
	if m.runOutputTop < 0 {
		m.runOutputTop = 0
	}
	if m.runOutputTop > maxTop {
		m.runOutputTop = maxTop
	}
}

// renderEntryOptions draws the entry-point selector as a row of
// labels with the selected one highlighted — the same "row of choices,
// one picked out" shape the tab bar itself uses, just inline rather
// than across the top of the screen. "" (the bare `store`) renders as
// "(default)", matching --store's own convention that omitting it
// means the bare store recipe.
func renderEntryOptions(options []string, selected int) string {
	parts := make([]string, len(options))
	for i, o := range options {
		label := o
		if label == "" {
			label = "(default)"
		}
		if i == selected {
			parts[i] = styleSelectedRow.Render(label)
		} else {
			parts[i] = styleMuted.Render(label)
		}
	}
	return strings.Join(parts, "   ")
}
