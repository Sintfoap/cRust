package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Sintfoap/cRust/internal/debugger"
	"github.com/Sintfoap/cRust/internal/interpreter"
	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/object"
	"github.com/Sintfoap/cRust/internal/parser"
)

// viewFor records src (running it as a plain top-to-bottom program, no
// entry-point resolution needed for these model-level tests) and
// returns a debugView the same shape runDebug builds, for driving
// debugModel directly — the same "test the model, not a real
// terminal" approach the bubbletea docs recommend and this project's
// own REPL TTY tests already use elsewhere.
func viewFor(t *testing.T, src string) *debugView {
	t.Helper()
	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parser errors: %v", errs)
	}
	rec := debugger.NewRecorder(0)
	interp := interpreter.New(&bytes.Buffer{}, strings.NewReader(""))
	interp.Trace = rec
	interp.Eval(program, object.NewEnvironment())
	return &debugView{path: "x.crust", rec: rec}
}

func TestDebugModelStartsOnTimeTab(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	if m.active != tabTime {
		t.Errorf("active = %v, want tabTime", m.active)
	}
}

func TestDebugModelTabSwitchesBackAndForth(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	want := []tab{tabMemory, tabStepper, tabEditor, tabRun, tabTime}
	for _, w := range want {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = next.(debugModel)
		if m.active != w {
			t.Fatalf("after tab: active = %v, want %v", m.active, w)
		}
	}
}

func TestDebugModelShiftTabGoesBackward(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = next.(debugModel)
	if m.active != tabRun {
		t.Errorf("active = %v, want tabRun (wrapped backward from Time)", m.active)
	}
}

func TestDebugModelSwitchingToEditorTabAutoLaunchesNvim(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.view.path = writeDebugFile(t, "x = 1\n")
	m.active = tabStepper
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Stepper -> Editor
	if cmd == nil {
		t.Error("switching to the Editor tab should return a non-nil Cmd (auto-launch nvim)")
	}
}

func TestDebugModelSwitchingToOtherTabsDoesNotLaunchNvim(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyTab}) // Time -> Memory
	if cmd != nil {
		t.Error("switching to the Memory tab should not launch nvim")
	}
}

func TestDebugModelQuitReturnsQuitCmd(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd for q")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("cmd() = %T, want tea.QuitMsg", msg)
	}
}

func TestDebugModelWindowSizeUpdatesDimensions(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = next.(debugModel)
	if m.width != 100 || m.height != 40 {
		t.Errorf("width, height = %d, %d, want 100, 40", m.width, m.height)
	}
}

func TestDebugModelCursorMovesWithinBounds(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1\ny = 2\nz = 3\n"))
	m.active = tabStepper
	if len(m.rows) < 3 {
		t.Fatalf("got %d rows, want at least 3", len(m.rows))
	}
	// Up from 0 stays at 0.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = next.(debugModel)
	if m.cursor != 0 {
		t.Errorf("cursor after up-at-top = %d, want 0", m.cursor)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(debugModel)
	if m.cursor != 1 {
		t.Errorf("cursor after down = %d, want 1", m.cursor)
	}
	// Down past the end clamps to the last row.
	for i := 0; i < len(m.rows)+5; i++ {
		next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = next.(debugModel)
	}
	if m.cursor != len(m.rows)-1 {
		t.Errorf("cursor after overshooting down = %d, want %d", m.cursor, len(m.rows)-1)
	}
}

func TestDebugModelCursorIgnoredOnTimeTab(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1\ny = 2\n"))
	// active stays tabTime (the default).
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(debugModel)
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (down should do nothing on the Time tab)", m.cursor)
	}
}

func TestDebugModelToggleFoldExpandsAndCollapses(t *testing.T) {
	m := newDebugModel(viewFor(t, `
total = 0
knead n in [1, 2, 3, 4, 5] {
    total += n
}
`))
	m.active = tabStepper
	before := len(m.rows)

	// Find the folded row and put the cursor on it.
	idx := -1
	for i, r := range m.rows {
		if r.node.Folded {
			idx = i
		}
	}
	if idx == -1 {
		t.Fatal("expected a folded row for a 5-lap loop")
	}
	m.cursor = idx

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(debugModel)
	if len(m.rows) <= before {
		t.Errorf("rows after expanding fold = %d, want more than %d", len(m.rows), before)
	}

	// Toggling again collapses it back.
	m.cursor = idx
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(debugModel)
	if len(m.rows) != before {
		t.Errorf("rows after collapsing fold = %d, want back to %d", len(m.rows), before)
	}
}

func TestDebugModelToggleFoldOnNonFoldedRowIsANoOp(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabStepper
	before := len(m.rows)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(debugModel)
	if len(m.rows) != before {
		t.Errorf("rows changed on a non-folded row: %d -> %d", before, len(m.rows))
	}
}

func TestDebugModelViewContainsAllTabLabels(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.width, m.height = 100, 30
	out := m.View()
	for _, label := range []string{"Time", "Memory", "Stepper", "Editor", "Run"} {
		if !strings.Contains(out, label) {
			t.Errorf("View() missing tab label %q: %q", label, out)
		}
	}
}

func TestDebugModelViewStepperShowsStatements(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1\ny = 2\n"))
	m.active = tabStepper
	m.width, m.height = 100, 30
	out := m.View()
	if !strings.Contains(out, "x = 1") || !strings.Contains(out, "y = 2") {
		t.Errorf("View() missing recorded statements: %q", out)
	}
}

func TestDebugModelViewTimeShowsFamilyNames(t *testing.T) {
	m := newDebugModel(viewFor(t, `
recipe double(x) {
    serve x * 2
}
y = double(21)
`))
	m.width, m.height = 100, 30
	out := m.View()
	if !strings.Contains(out, "double(...)") {
		t.Errorf("View() (Time tab) missing the double(...) family: %q", out)
	}
}

func TestDebugModelViewMemoryShowsFamilyNames(t *testing.T) {
	// Uses a List result, not a scalar -- trace.SizeOf only sizes
	// container-like values, so a purely-integer recipe wouldn't show
	// up on the memory chart at all (there'd be nothing to rank).
	m := newDebugModel(viewFor(t, `
recipe makeList(x) {
    serve [x, x, x]
}
y = makeList(21)
`))
	m.active = tabMemory
	m.width, m.height = 100, 30
	out := m.View()
	if !strings.Contains(out, "makeList(...)") {
		t.Errorf("View() (Memory tab) missing the makeList(...) family: %q", out)
	}
}

func TestDebugModelViewOnEmptyRecording(t *testing.T) {
	rec := debugger.NewRecorder(0)
	m := newDebugModel(&debugView{path: "empty.crust", rec: rec})
	m.active = tabStepper
	m.width, m.height = 100, 30
	// Must not panic on an empty recording.
	_ = m.View()
}

func TestBuildRowsRespectsCollapsedFold(t *testing.T) {
	view := viewFor(t, `
total = 0
knead n in [1, 2, 3, 4, 5] {
    total += n
}
`)
	rows := buildRows(view.rec.Roots(), 0, map[*debugger.TraceNode]bool{})
	for _, r := range rows {
		if r.node.Folded {
			// A collapsed fold row is present, but its children (the
			// individual laps) must not be, since expanded is empty.
			return
		}
	}
	t.Fatal("expected a folded row in a 5-lap loop's rows")
}

func TestSlowestStepFindsTheRightNode(t *testing.T) {
	view := viewFor(t, "x = 1\ny = 2\n")
	rows := buildRows(view.rec.Roots(), 0, map[*debugger.TraceNode]bool{})
	slow, ok := slowestStep(rows, view.timing())
	if !ok {
		t.Fatal("expected a slowest step to be found")
	}
	if slow.IsFrame() {
		t.Error("slowestStep returned a frame, want a step")
	}
}

func TestSlowestStepOnEmptyRows(t *testing.T) {
	view := viewFor(t, "")
	_, ok := slowestStep(nil, view.timing())
	if ok {
		t.Error("expected no slowest step for an empty row set")
	}
}

func TestDebugModelInitReturnsNilCmd(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	if cmd := m.Init(); cmd != nil {
		t.Errorf("Init() = %v, want nil (nothing async to kick off)", cmd)
	}
}

func TestDebugModelIgnoresUnknownKeys(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("z")})
	if cmd != nil {
		t.Errorf("unexpected cmd for an unbound key: %v", cmd)
	}
	if next.(debugModel).active != tabTime {
		t.Error("unbound key should not change tabs")
	}
}

func TestDebugModelScrollsWhenCursorLeavesVisibleWindow(t *testing.T) {
	var src strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&src, "x%d = %d\n", i, i)
	}
	m := newDebugModel(viewFor(t, src.String()))
	m.active = tabStepper
	m.width, m.height = 100, 15 // a small body height forces scrolling
	for i := 0; i < len(m.rows); i++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = next.(debugModel)
	}
	if m.top == 0 {
		t.Error("expected top to have scrolled down for a tall tree in a short window")
	}
	// And scrolling back up all the way returns top to 0.
	for i := 0; i < len(m.rows); i++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
		m = next.(debugModel)
	}
	if m.top != 0 {
		t.Errorf("top after scrolling back to the start = %d, want 0", m.top)
	}
}

func TestViewTimeShowsTimeChartOnly(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	out := m.viewTime()
	if !strings.Contains(out, "time by function") {
		t.Errorf("viewTime() missing its chart title: %q", out)
	}
	if strings.Contains(out, "memory by function") {
		t.Errorf("viewTime() should not include the memory chart: %q", out)
	}
}

func TestViewMemoryShowsMemoryChartOnly(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	out := m.viewMemory()
	if !strings.Contains(out, "memory by function") {
		t.Errorf("viewMemory() missing its chart title: %q", out)
	}
	if strings.Contains(out, "time by function") {
		t.Errorf("viewMemory() should not include the time chart: %q", out)
	}
}

func TestViewMemoryStatsShowsLargestValue(t *testing.T) {
	m := newDebugModel(viewFor(t, `x = "a very large string value here"
y = 1`))
	out := m.viewMemoryStats()
	if !strings.Contains(out, "largest single value") {
		t.Errorf("viewMemoryStats() = %q, want a largest-single-value line", out)
	}
}

func TestLargestValueOnEmptyRows(t *testing.T) {
	if _, ok := largestValue(nil); ok {
		t.Error("expected no largest value for an empty row set")
	}
}

func TestTruncatedNoteMentionsCapWhenTruncated(t *testing.T) {
	// A 1-step cap on a program with more than one statement guarantees
	// truncation.
	l := lexer.New("knead i in 0.<5 { x = i }")
	p := parser.New(l)
	program := p.ParseProgram()
	rec := debugger.NewRecorder(1)
	interp := interpreter.New(&bytes.Buffer{}, strings.NewReader(""))
	interp.Trace = rec
	interp.Eval(program, object.NewEnvironment())
	if !rec.Truncated() {
		t.Fatal("expected the recording to be truncated")
	}
	if got := truncatedNote(rec); !strings.Contains(got, "capped") {
		t.Errorf("truncatedNote() = %q, want it to mention capped", got)
	}
}

func TestRenderRowStylesAFailedStep(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1 / 0"))
	m.active = tabStepper
	out := m.viewStepper()
	if !strings.Contains(out, "x = (1 / 0)") {
		t.Errorf("viewStepper() missing the failing statement: %q", out)
	}
}

func TestTruncateRunesSingleRuneBudget(t *testing.T) {
	if got := truncateRunes("hello", 1); got != "h" {
		t.Errorf("truncateRunes(_, 1) = %q, want %q", got, "h")
	}
}

func TestBuildRowsAddsClosingRowForFrameWithChildren(t *testing.T) {
	view := viewFor(t, `
recipe double(x) {
    serve x * 2
}
y = double(21)
`)
	rows := buildRows(view.rec.Roots(), 0, map[*debugger.TraceNode]bool{})
	var openIdx, closeIdx = -1, -1
	for i, r := range rows {
		if r.node.IsFrame() && r.node.Frame == "double(...)" {
			if r.closing {
				closeIdx = i
			} else {
				openIdx = i
			}
		}
	}
	if openIdx == -1 || closeIdx == -1 {
		t.Fatalf("expected both an opening and closing row for double(...), got open=%d close=%d", openIdx, closeIdx)
	}
	if closeIdx <= openIdx {
		t.Errorf("closing row at %d, want it after the opening row at %d", closeIdx, openIdx)
	}
	if rows[openIdx].depth != rows[closeIdx].depth {
		t.Errorf("closing row depth = %d, want it to match the opening row's depth %d", rows[closeIdx].depth, rows[openIdx].depth)
	}
}

func TestBuildRowsNoClosingRowForLeafStep(t *testing.T) {
	view := viewFor(t, "x = 1")
	rows := buildRows(view.rec.Roots(), 0, map[*debugger.TraceNode]bool{})
	for _, r := range rows {
		if r.closing {
			t.Errorf("unexpected closing row for a leaf statement: %+v", r)
		}
	}
}

func TestBuildRowsNoClosingRowWhenFoldCollapsed(t *testing.T) {
	view := viewFor(t, `
total = 0
knead n in [1, 2, 3, 4, 5] {
    total += n
}
`)
	rows := buildRows(view.rec.Roots(), 0, map[*debugger.TraceNode]bool{})
	for _, r := range rows {
		if r.node.Folded && r.closing {
			t.Error("a collapsed fold row should have no closing row, since its children aren't shown")
		}
	}
}

func TestRenderClosingRowHighlightsCursor(t *testing.T) {
	m := newDebugModel(viewFor(t, `
recipe double(x) {
    serve x * 2
}
y = double(21)
`))
	m.active = tabStepper
	closeIdx := -1
	for i, r := range m.rows {
		if r.closing && r.node.IsFrame() && r.node.Frame == "double(...)" {
			closeIdx = i
		}
	}
	if closeIdx == -1 {
		t.Fatal("expected a closing row for the double(...) frame")
	}
	m.cursor = closeIdx
	out := m.renderClosingRow(closeIdx)
	if !strings.Contains(out, "// end double(...)") {
		t.Errorf("renderClosingRow() = %q, want it to mention // end double(...)", out)
	}
}

func TestClosingLabelStripsContinuationMarkers(t *testing.T) {
	cases := map[string]string{
		"knead r in (0.<5) { …": "knead r in (0.<5)",
		"recipe double(x) { …":  "recipe double(x)",
		"step(...)":             "step(...)",
	}
	for in, want := range cases {
		if got := closingLabel(in); got != want {
			t.Errorf("closingLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClosingLabelTruncatesLongLabels(t *testing.T) {
	long := strings.Repeat("x", 100)
	got := closingLabel(long)
	if len([]rune(got)) != 50 {
		t.Errorf("closingLabel(long) length = %d, want 50", len([]rune(got)))
	}
}

func TestClampHeightNoLimitReturnsInput(t *testing.T) {
	s := "a\nb\nc"
	if got := clampHeight(s, 0); got != s {
		t.Errorf("clampHeight(_, 0) = %q, want unchanged input", got)
	}
}

func TestClampHeightUnderLimitReturnsInput(t *testing.T) {
	s := "a\nb\nc"
	if got := clampHeight(s, 10); got != s {
		t.Errorf("clampHeight(_, 10) = %q, want unchanged input", got)
	}
}

func TestClampHeightOverLimitTruncatesWithNote(t *testing.T) {
	s := "a\nb\nc\nd\ne"
	got := clampHeight(s, 3)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("clampHeight(_, 3) produced %d lines, want 3", len(lines))
	}
	if lines[0] != "a" || lines[1] != "b" {
		t.Errorf("clampHeight(_, 3) kept lines = %v, want the first two original lines preserved", lines[:2])
	}
	if !strings.Contains(lines[2], "grow the terminal") {
		t.Errorf("clampHeight(_, 3) last line = %q, want a truncation note", lines[2])
	}
}

func TestDebugModelViewNeverExceedsHeight(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.width, m.height = 80, 24
	out := m.View()
	if got := len(strings.Split(out, "\n")); got > m.height {
		t.Errorf("View() produced %d lines, want at most %d", got, m.height)
	}
}

func TestSliceIndexLastBucketFallback(t *testing.T) {
	bounds := []float64{0, 0.5, 1.0}
	// frac == 1.0 (a closed upper edge floating-point can actually
	// produce) never satisfies `frac < bounds[i]` for any i, so this
	// must fall through to the last bucket rather than panic or
	// under-index.
	if got := sliceIndex(bounds, 1.0); got != 1 {
		t.Errorf("sliceIndex(_, 1.0) = %d, want 1 (the last bucket)", got)
	}
}

// TestStdinNoNamerDoesNotSatisfyCancelreaderFileShape is half of
// stdinNoNamer's whole point: bubbletea's cancelreader dependency
// decides whether to take its epoll-based reader (broken under some
// WSL setups -- "add reader to epoll interest list") purely via a
// type assertion to an unexported interface shaped like
// io.ReadWriteCloser + Fd() uintptr + Name() string. This mirrors
// that exact shape locally (avoiding a direct test dependency on
// cancelreader's internals) to prove stdinNoNamer defeats it, while
// the underlying *os.File still would satisfy it — confirming the
// wrapper is what actually changes cancelreader's decision, not an
// accident of some other difference between the two.
func TestStdinNoNamerDoesNotSatisfyCancelreaderFileShape(t *testing.T) {
	type cancelreaderFileShape interface {
		io.ReadWriteCloser
		Fd() uintptr
		Name() string
	}

	f, err := os.CreateTemp(t.TempDir(), "stdin-no-namer-test")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if _, ok := interface{}(f).(cancelreaderFileShape); !ok {
		t.Fatal("test setup: *os.File should satisfy the mirrored shape")
	}
	if _, ok := interface{}(stdinNoNamer{f}).(cancelreaderFileShape); ok {
		t.Error("stdinNoNamer should not satisfy the cancelreader File shape -- that's half the reason it exists")
	}
}

// TestStdinNoNamerSatisfiesTermFileShape is the other half: bubbletea's
// own initInput (tty_unix.go) decides whether to call term.MakeRaw --
// disabling terminal echo and line buffering so keypresses reach
// bubbletea as discrete events instead of the OS echoing them straight
// into the terminal -- via a type assertion to
// github.com/charmbracelet/x/term's File, a narrower shape than
// cancelreader's: io.ReadWriteCloser + Fd() uintptr, no Name(). An
// earlier wrapper (stdinOnlyReader) hid Fd() too, defeating this check
// right along with cancelreader's and leaving raw mode never engaged --
// the exact bug stdinNoNamer's Fd() forwarding method exists to fix.
func TestStdinNoNamerSatisfiesTermFileShape(t *testing.T) {
	type termFileShape interface {
		io.ReadWriteCloser
		Fd() uintptr
	}

	f, err := os.CreateTemp(t.TempDir(), "stdin-no-namer-test")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if _, ok := interface{}(stdinNoNamer{f}).(termFileShape); !ok {
		t.Error("stdinNoNamer should satisfy the term.File shape so bubbletea's raw-mode detection still engages")
	}
}

func TestStdinNoNamerReadsThrough(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stdin-no-namer-read-test")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	r := stdinNoNamer{f}
	buf := make([]byte, 5)
	n, err := r.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if n != 5 || string(buf) != "hello" {
		t.Errorf("Read() = (%d, %q), want (5, %q)", n, buf, "hello")
	}
}
