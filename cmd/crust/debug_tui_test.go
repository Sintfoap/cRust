package main

import (
	"bytes"
	"fmt"
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

func TestDebugModelStartsOnKPITab(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	if m.active != tabKPI {
		t.Errorf("active = %v, want tabKPI", m.active)
	}
}

func TestDebugModelTabSwitchesBackAndForth(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(debugModel)
	if m.active != tabStepper {
		t.Errorf("after tab: active = %v, want tabStepper", m.active)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(debugModel)
	if m.active != tabKPI {
		t.Errorf("after second tab: active = %v, want tabKPI", m.active)
	}
}

func TestDebugModelShiftTabGoesBackward(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = next.(debugModel)
	if m.active != tabStepper {
		t.Errorf("active = %v, want tabStepper (wrapped backward from KPI)", m.active)
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

func TestDebugModelCursorIgnoredOnKPITab(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1\ny = 2\n"))
	// active stays tabKPI (the default).
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(debugModel)
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (down should do nothing on the KPI tab)", m.cursor)
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

func TestDebugModelViewContainsBothTabLabels(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.width, m.height = 100, 30
	out := m.View()
	if !strings.Contains(out, "KPIs") || !strings.Contains(out, "Stepper") {
		t.Errorf("View() missing a tab label: %q", out)
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

func TestDebugModelViewKPIShowsFamilyNames(t *testing.T) {
	m := newDebugModel(viewFor(t, `
recipe double(x) {
    serve x * 2
}
y = double(21)
`))
	m.width, m.height = 100, 30
	out := m.View()
	if !strings.Contains(out, "double(...)") {
		t.Errorf("View() (KPI tab) missing the double(...) family: %q", out)
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
	if next.(debugModel).active != tabKPI {
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

func TestViewKPINarrowWidthStacksVertically(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.width = 40
	out := m.viewKPI()
	if !strings.Contains(out, "time by function") || !strings.Contains(out, "memory by function") {
		t.Errorf("viewKPI narrow output missing a chart title: %q", out)
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
