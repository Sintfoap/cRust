// The Live tab (debug_tui.go's tabLive): step through a real, running
// program one statement at a time, or set breakpoints and let it run
// up to them — genuinely *live* debugging, unlike the Stepper tab's
// after-the-fact recording. 'r' starts (or restarts) the file with the
// Run tab's currently selected entry point and input file (the exact
// same settings the Bench tab already reuses, read fresh at run time,
// not a separate copy); 'b' toggles a breakpoint on the source line
// under the cursor; 's' single-steps; 'c' continues to the next
// breakpoint (or the end); 'x' stops the run early.
//
// The mechanism is internal/debugger.LiveTracer: a trace.Tracer whose
// Step call blocks — on the interpreter's own goroutine, not this
// TUI's — until the UI says what to do next. "Paused" is not a UI
// state this file tracks independently of reality; it *is* that
// goroutine sitting blocked inside Step, waiting on LiveTracer.Resume.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Sintfoap/cRust/internal/debugger"
	"github.com/Sintfoap/cRust/internal/interpreter"
	"github.com/Sintfoap/cRust/internal/object"
)

// liveOutputBuffer collects a live run's own stdout (deliver, etc.)
// safely across two goroutines: the interpreter's own (writing, as the
// program executes) and the TUI's (reading it fresh on every redraw,
// the same content m.runOutput shows on the Run tab, just updated
// incrementally here instead of all at once at the end).
type liveOutputBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *liveOutputBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *liveOutputBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// liveDoneMsg reports a live run's final outcome, whichever of the
// three ways it ends: LiveStop/RequestStop (Stopped), a cRust parse/
// runtime error or an unresolvable entry point (Failed, with Message
// describing it — the same simple bool-plus-text shape m.runFailed/
// m.runOutput already use on the Run tab), or a clean finish (neither
// set). gen ties this back to the run that produced it — see
// debugModel.liveGen's own doc comment on why a stale message from an
// abandoned run must never be applied to whatever's current.
type liveDoneMsg struct {
	gen     int
	Stopped bool
	Failed  bool
	Message string
}

// livePauseMsg reports one pause of a still-running live run.
type livePauseMsg struct {
	gen int
	debugger.LivePause
}

// liveStartMsg reports the outcome of trying to start a live run:
// either it never got going at all (err — a parse failure, or the Run
// tab's own input file couldn't be read) or it's now running, with the
// handles (tracer/output/done) the rest of this file drives it through
// from here, plus whichever of two things happened first: an immediate
// pause (the ordinary case — LiveTracer always starts in single-step
// mode, so the very first statement pauses) or the run finishing
// before the UI ever got a chance to see it pause (an empty top-level
// body with no entry point either — rare, but not impossible).
type liveStartMsg struct {
	gen    int
	err    error
	tracer *debugger.LiveTracer
	output *liveOutputBuffer
	done   <-chan liveDoneMsg
	pause  *debugger.LivePause
	result *liveDoneMsg
}

// startLiveRun parses path and launches it on its own goroutine under
// a fresh LiveTracer seeded with breakpoints, returning immediately —
// the same "kick off the real work, report back on a channel" split
// buildDebugView's callers already use for a traced run, just with a
// pausable Tracer instead of a Recorder. The three ways the goroutine
// can end (see liveDoneMsg) are all funneled through done exactly
// once: a LiveStop/RequestStop unwinds via panic(LiveStopSignal{}),
// recovered here as Stopped rather than left to crash the goroutine,
// mirroring run.go's own top-level recover()'s "not the primary error
// path, just an escape hatch" reasoning for a Go panic that isn't a
// cRust runtime Error — any *other* panic is reported as Failed
// instead of silently swallowed, since that would be a real
// interpreter bug worth seeing.
func startLiveRun(path string, opts debugOptions, stdin io.Reader, breakpoints map[int]bool) (*debugger.LiveTracer, *liveOutputBuffer, <-chan liveDoneMsg, error) {
	program, err := parseDebugFile(path)
	if err != nil {
		return nil, nil, nil, err
	}

	lt := debugger.NewLiveTracer()
	lt.SetBreakpoints(breakpoints)
	out := &liveOutputBuffer{}
	done := make(chan liveDoneMsg, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(debugger.LiveStopSignal); ok {
					done <- liveDoneMsg{Stopped: true}
					return
				}
				done <- liveDoneMsg{Failed: true, Message: fmt.Sprintf("internal error: %v", r)}
			}
		}()

		interp := interpreter.New(out, stdin)
		interp.Trace = lt
		env := object.NewEnvironment()

		if errObj, failed := interp.Eval(program, env).(*object.Error); failed {
			done <- liveDoneMsg{Failed: true, Message: fmt.Sprintf("%d:%d: %s", errObj.Line, errObj.Col, errObj.Message)}
			return
		}
		if err := runDebugEntryPoint(interp, env, program, opts.Store); err != nil {
			done <- liveDoneMsg{Failed: true, Message: err.Error()}
			return
		}
		done <- liveDoneMsg{}
	}()

	return lt, out, done, nil
}

// readLiveSource reads path's lines for the Live tab's source view, or
// nil if it can't be read right now — the same "degrade quietly rather
// than fail the whole tab" choice scanEntryPoints/listCrustFiles
// already make for their own filesystem reads.
func readLiveSource(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	text := strings.TrimSuffix(string(data), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// startLiveCmd builds the Cmd 'r' fires: read the Run tab's input file
// (if any) and launch startLiveRun against it, reporting whichever of
// an immediate pause or an immediate finish happens first. gen is
// captured at construction time (by the caller, right before
// incrementing m.liveGen) so the eventual liveStartMsg can be told
// apart from a later run's, or a run abandoned by switching files in
// the meantime (m.liveGen's own doc comment).
func (m debugModel) startLiveCmd(gen int) tea.Cmd {
	path := m.view.path
	store := m.selectedRunEntry()
	inputPath := strings.TrimSpace(m.runInput.String())
	breakpoints := m.liveBreakpoints
	return func() tea.Msg {
		var data []byte
		if inputPath != "" {
			d, err := os.ReadFile(inputPath)
			if err != nil {
				return liveStartMsg{gen: gen, err: err}
			}
			data = d
		}

		lt, out, done, err := startLiveRun(path, debugOptions{Store: store}, bytes.NewReader(data), breakpoints)
		if err != nil {
			return liveStartMsg{gen: gen, err: err}
		}
		select {
		case p := <-lt.Paused:
			return liveStartMsg{gen: gen, tracer: lt, output: out, done: done, pause: &p}
		case res := <-done:
			res.gen = gen
			return liveStartMsg{gen: gen, tracer: lt, output: out, done: done, result: &res}
		}
	}
}

// liveResumeCmd sends resume on the live run's own Resume channel and
// then keeps listening for whatever happens next (another pause, or
// the run finishing) — reissued after every pause the same way
// waitForLiveEvent's shape generalizes across every channel-driven Cmd
// in this file. Callers must only invoke this immediately after
// confirming m.liveRunning && m.livePaused (handleKey's own 'c'/'s'
// cases do, right before flipping livePaused off for the immediate
// "now running" feedback) — LiveTracer.Step only reads from Resume
// while it's blocked at a pause, and Resume is unbuffered, so sending
// here with nothing on the other end actually listening would block
// this goroutine forever instead of erroring.
func (m debugModel) liveResumeCmd(resume debugger.LiveResume) tea.Cmd {
	lt, done, gen := m.liveTracer, m.liveDone, m.liveGen
	return func() tea.Msg {
		lt.Resume <- resume
		select {
		case p := <-lt.Paused:
			return livePauseMsg{gen: gen, LivePause: p}
		case res := <-done:
			res.gen = gen
			return res
		}
	}
}

// handleLiveStart applies a liveStartMsg. A stale one (from a run since
// abandoned — switching files bumps m.liveGen) is silently ignored,
// the same guard handleLivePause/handleLiveDone use.
func (m debugModel) handleLiveStart(msg liveStartMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.liveGen {
		return m, nil
	}
	if msg.err != nil {
		m.liveStatus = msg.err.Error()
		return m, nil
	}
	m.liveRunning = true
	m.liveTracer = msg.tracer
	m.liveOutput = msg.output
	m.liveDone = msg.done
	switch {
	case msg.pause != nil:
		m.applyLivePause(*msg.pause)
	case msg.result != nil:
		m.applyLiveDone(*msg.result)
	}
	return m, nil
}

func (m debugModel) handleLivePause(msg livePauseMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.liveGen {
		return m, nil
	}
	m.applyLivePause(msg.LivePause)
	return m, nil
}

func (m debugModel) handleLiveDone(msg liveDoneMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.liveGen {
		return m, nil
	}
	m.applyLiveDone(msg)
	return m, nil
}

func (m *debugModel) applyLivePause(p debugger.LivePause) {
	m.liveStatus = ""
	m.livePaused = true
	m.livePausedAt = p.Line
	m.liveLabel = p.Label
	m.liveEnv = p.Env
	if p.Line > 0 {
		m.liveCursor = p.Line - 1
		m.scrollLiveToCursor()
	}
}

func (m *debugModel) applyLiveDone(res liveDoneMsg) {
	m.liveRunning = false
	m.livePaused = false
	m.livePausedAt = 0
	switch {
	case res.Stopped:
		m.liveStatus = "stopped"
	case res.Failed:
		m.liveStatus = res.Message
	default:
		m.liveStatus = "finished"
	}
}

// maybeRefreshLive rescans m.view.path's source the moment a tab
// switch lands on Live, mirroring maybeRefreshNav's own "switching to
// the tab is the action" shape — a no-op for every other tab. Only
// reloads the source text (for browsing/breakpoint placement); it
// never touches a run already in progress, which keeps going
// regardless of which tab happens to be showing.
func (m *debugModel) maybeRefreshLive() {
	if m.active != tabLive {
		return
	}
	m.liveSource = readLiveSource(m.view.path)
	if m.liveCursor >= len(m.liveSource) {
		m.liveCursor = 0
		m.liveTop = 0
	}
}

// moveLiveCursor moves liveCursor by delta, clamped at both ends (not
// wrapping) — the same "a long list where wrapping to the top would be
// more surprising than useful" reasoning moveNavCursor already
// documents, just over source lines instead of files.
func (m *debugModel) moveLiveCursor(delta int) {
	if len(m.liveSource) == 0 {
		return
	}
	m.liveCursor += delta
	if m.liveCursor < 0 {
		m.liveCursor = 0
	}
	if m.liveCursor >= len(m.liveSource) {
		m.liveCursor = len(m.liveSource) - 1
	}
	m.scrollLiveToCursor()
}

// scrollLiveToCursor slides liveTop (the first visible source line) to
// keep liveCursor on screen — clampAndScrollCursor's own logic, over
// the Live tab's source view instead of the Stepper's row list.
func (m *debugModel) scrollLiveToCursor() {
	visible := m.liveBodyHeight()
	if m.liveCursor < m.liveTop {
		m.liveTop = m.liveCursor
	}
	if m.liveCursor >= m.liveTop+visible {
		m.liveTop = m.liveCursor - visible + 1
	}
}

// toggleLiveBreakpoint flips the breakpoint on the source line under
// liveCursor, persists the new set (debug_state.go), and — when a run
// is currently active — pushes it straight to the live LiveTracer too,
// so toggling a breakpoint mid-run (even while paused somewhere else
// entirely) takes effect on the very next statement, not just the next
// time 'r' is pressed.
func (m *debugModel) toggleLiveBreakpoint() {
	if len(m.liveSource) == 0 {
		return
	}
	line := m.liveCursor + 1
	if m.liveBreakpoints[line] {
		delete(m.liveBreakpoints, line)
	} else {
		m.liveBreakpoints[line] = true
	}
	saveLiveBreakpointsBestEffort(m.view.path, m.liveBreakpoints)
	if m.liveTracer != nil {
		m.liveTracer.SetBreakpoints(m.liveBreakpoints)
	}
}

// maxLiveWatchLines/maxLiveOutputLines cap the Live tab's variables and
// output panels — the same fixed-worst-case reservation shape
// maxWatchLines/maxDetailLines already established for the Stepper
// tab's own panels, so liveBodyHeight's budget can never drift out of
// sync with what actually renders.
const (
	maxLiveWatchLines  = 6
	maxLiveOutputLines = 6
)

// liveExtraLines is how many lines viewLive prints beyond the source
// view itself: the status line, plus the variables panel while paused,
// plus the output panel once the run has produced any — computed here
// rather than duplicated inside viewLive, the same reasoning
// stepperExtraLines already documents for the Stepper tab.
func (m debugModel) liveExtraLines() int {
	extra := 2 // status line + blank
	if m.livePaused {
		extra += maxLiveWatchLines + 2
	}
	if m.liveOutput != nil && m.liveOutput.String() != "" {
		extra += maxLiveOutputLines + 2
	}
	return extra
}

// liveBodyHeight is how many source lines fit on screen at once, the
// same budget-below-the-chrome shape stepperBodyHeight already
// establishes.
func (m debugModel) liveBodyHeight() int {
	h := m.height - 6 - m.liveExtraLines()
	if h < 3 {
		h = 3
	}
	return h
}

// liveStatusLine is the Live tab's one-line summary, above the source
// view.
func (m debugModel) liveStatusLine() string {
	switch {
	case m.liveRunning && m.livePaused:
		return fmt.Sprintf("paused at line %d: %s", m.livePausedAt, m.liveLabel)
	case m.liveRunning:
		return "running…"
	case m.liveStatus != "":
		return m.liveStatus
	default:
		return "not started"
	}
}

// renderLiveLine draws one source line: a breakpoint dot, the cursor
// marker, the line number, and the source text itself — one lipgloss
// style for the whole line (the same single-style-per-row shape
// renderRow already uses, rather than nesting styled spans inside each
// other), in priority order: the currently paused-at line outranks
// everything else, then the cursor, then a plain breakpoint.
func (m debugModel) renderLiveLine(i int) string {
	line := i + 1
	marker := "  "
	if i == m.liveCursor {
		marker = "> "
	}
	bp := " "
	if m.liveBreakpoints[line] {
		bp = "●"
	}
	text := fmt.Sprintf("%s%s %4d  %s", marker, bp, line, m.liveSource[i])

	switch {
	case m.livePaused && line == m.livePausedAt:
		return styleSelectedRow.Render(text)
	case i == m.liveCursor:
		return lipgloss.NewStyle().Bold(true).Render(text)
	case m.liveBreakpoints[line]:
		return styleError.Render(text)
	default:
		return text
	}
}

// viewLiveWatch renders the variables visible at the current pause —
// viewStepperWatch's own shape (sorted names, shortInspect values,
// capped at maxLiveWatchLines), just reading m.liveEnv directly instead
// of a selected row's Step.Env, since the Live tab has exactly one
// "current" point in the program rather than a whole tree to pick a
// row from.
func (m debugModel) viewLiveWatch() string {
	if len(m.liveEnv) == 0 {
		return styleMuted.Render("(no variables visible yet)")
	}
	names := make([]string, 0, len(m.liveEnv))
	for name := range m.liveEnv {
		names = append(names, name)
	}
	slices.Sort(names)

	shown := names
	truncated := 0
	if len(shown) > maxLiveWatchLines {
		truncated = len(shown) - maxLiveWatchLines
		shown = shown[:maxLiveWatchLines]
	}

	var b strings.Builder
	b.WriteString(styleTitle.Render(fmt.Sprintf("variables (%d)", len(names))))
	b.WriteByte('\n')
	for _, name := range shown {
		fmt.Fprintf(&b, "%s = %s\n", styleMuted.Render(name), shortInspect(m.liveEnv[name]))
	}
	if truncated > 0 {
		b.WriteString(styleFaint.Render(fmt.Sprintf("… and %d more\n", truncated)))
	}
	return b.String()
}

// tailLines returns s's last n lines (dropping any earlier ones with a
// count of how many), for the Live tab's output panel — a tail rather
// than wrapStepperDetail's head-first truncation, since the most
// recently produced output is the one worth seeing while a program is
// still running.
func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	head := len(lines) - n
	return styleFaint.Render(fmt.Sprintf("… (%d earlier line(s))", head)) + "\n" + strings.Join(lines[head:], "\n")
}

// viewLive is the Live tab: a status line, the source file (scrollable,
// breakpoints and the cursor marked), and — once a run has produced
// them — the variables-in-scope and output panels below it.
func (m debugModel) viewLive() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render(m.liveStatusLine()))
	b.WriteString("\n\n")

	if len(m.liveSource) == 0 {
		b.WriteString(styleMuted.Render("(no source to show)"))
		return b.String()
	}

	visible := m.liveBodyHeight()
	end := m.liveTop + visible
	if end > len(m.liveSource) {
		end = len(m.liveSource)
	}
	for i := m.liveTop; i < end; i++ {
		b.WriteString(m.renderLiveLine(i))
		b.WriteByte('\n')
	}

	if m.livePaused {
		b.WriteByte('\n')
		b.WriteString(m.viewLiveWatch())
	}
	if m.liveOutput != nil {
		if out := m.liveOutput.String(); out != "" {
			b.WriteByte('\n')
			b.WriteString(styleTitle.Render("output:"))
			b.WriteByte('\n')
			b.WriteString(tailLines(out, maxLiveOutputLines))
			b.WriteByte('\n')
		}
	}
	return b.String()
}
