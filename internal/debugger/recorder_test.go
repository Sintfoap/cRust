package debugger

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Sintfoap/cRust/internal/interpreter"
	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/object"
	"github.com/Sintfoap/cRust/internal/parser"
)

// record parses and evaluates input under a fresh Recorder, the same
// way `crust develop` will. Fails the test on a parse error.
func record(t *testing.T, input string, maxSteps int) *Recorder {
	t.Helper()
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parser errors for %q: %v", input, errs)
	}
	rec := NewRecorder(maxSteps)
	interp := interpreter.New(&bytes.Buffer{}, strings.NewReader(""))
	interp.Trace = rec
	interp.Eval(program, object.NewEnvironment())
	return rec
}

func TestRecorderFlatProgram(t *testing.T) {
	rec := record(t, "x = 1\ny = 2\n", 0)
	roots := rec.Roots()
	if len(roots) != 2 {
		t.Fatalf("got %d roots, want 2", len(roots))
	}
	if roots[0].IsFrame() || roots[0].Label() != "x = 1" {
		t.Errorf("roots[0] = %+v, want step x = 1", roots[0])
	}
	if roots[1].IsFrame() || roots[1].Label() != "y = 2" {
		t.Errorf("roots[1] = %+v, want step y = 2", roots[1])
	}
	if rec.Steps() != 2 {
		t.Errorf("Steps() = %d, want 2", rec.Steps())
	}
}

// TestStepLabelCollapsesMultiLineStatementsToOneLine is a regression
// test for a real display bug found while trying `crust develop` on an
// actual file: a recipe declaration's (or a knead/bake/order's) own
// String() renders its *entire* body, since that's what round-tripping
// source needs -- but a table row needs exactly one line, and the raw
// multi-line text broke the plain-text table's column alignment badly.
func TestStepLabelCollapsesMultiLineStatementsToOneLine(t *testing.T) {
	rec := record(t, `
recipe fib(n) {
    order (n < 2) {
        serve n
    }
    serve n
}
`, 0)
	roots := rec.Roots()
	label := roots[0].Step.Label()
	if strings.Contains(label, "\n") {
		t.Errorf("Label() = %q, contains a newline", label)
	}
	if label != "recipe fib(n) { …" {
		t.Errorf("Label() = %q, want %q", label, "recipe fib(n) { …")
	}
}

func TestRecorderRecipeCallNestsAFrame(t *testing.T) {
	rec := record(t, `
recipe double(x) {
    serve x * 2
}
y = double(21)
`, 0)
	roots := rec.Roots()
	if len(roots) != 2 {
		t.Fatalf("got %d roots, want 2 (the recipe decl, the call), roots = %+v", len(roots), roots)
	}
	callStep := roots[1]
	if callStep.IsFrame() || callStep.Label() != "y = double(21)" {
		t.Fatalf("roots[1] = %+v, want step y = double(21)", callStep)
	}
	if len(callStep.Children) != 1 || !callStep.Children[0].IsFrame() {
		t.Fatalf("call step's children = %+v, want one frame", callStep.Children)
	}
	frame := callStep.Children[0]
	if frame.Frame != "double(...)" {
		t.Errorf("frame label = %q, want %q", frame.Frame, "double(...)")
	}
	if len(frame.Children) != 1 || frame.Children[0].Label() != "serve (x * 2)" {
		t.Errorf("frame children = %+v, want one step serve (x * 2)", frame.Children)
	}
}

func TestRecorderFoldsRepeatedLoopLaps(t *testing.T) {
	rec := record(t, `
total = 0
knead n in [1, 2, 3, 4, 5] {
    total += n
}
`, 0)
	roots := rec.Roots()
	// roots[0] = "total = 0", roots[1] = the knead statement itself
	if len(roots) != 2 {
		t.Fatalf("got %d roots, want 2; roots = %+v", len(roots), roots)
	}
	kneadStep := roots[1]
	if len(kneadStep.Children) != 1 || !kneadStep.Children[0].Folded {
		t.Fatalf("knead step's children = %+v, want one folded row (5 laps >= foldFrom)", kneadStep.Children)
	}
	folded := kneadStep.Children[0]
	laps, ok := folded.Iterations()
	if !ok || laps != 5 {
		t.Errorf("Iterations() = %d, %v, want 5, true", laps, ok)
	}
}

func TestRecorderDoesNotFoldBelowThreshold(t *testing.T) {
	rec := record(t, `
total = 0
knead n in [1, 2] {
    total += n
}
`, 0)
	roots := rec.Roots()
	kneadStep := roots[1]
	if len(kneadStep.Children) != 2 {
		t.Fatalf("got %d children, want 2 laps kept unfolded; children = %+v", len(kneadStep.Children), kneadStep.Children)
	}
	for _, c := range kneadStep.Children {
		if c.Folded {
			t.Errorf("child %+v should not be folded (only 2 laps, foldFrom is %d)", c, foldFrom)
		}
	}
}

func TestRecorderTruncatesAtMaxSteps(t *testing.T) {
	rec := record(t, `
knead i in 0.<10 {
    x = i
}
`, 3)
	if !rec.Truncated() {
		t.Error("Truncated() = false, want true")
	}
	if rec.Steps() != 3 {
		t.Errorf("Steps() = %d, want 3 (the cap)", rec.Steps())
	}
	if !strings.Contains(rec.Summary(), "capped") {
		t.Errorf("Summary() = %q, want it to mention being capped", rec.Summary())
	}
}

func TestRecorderStepCarriesOutValueAndSize(t *testing.T) {
	rec := record(t, `xs = [1, 2, 3]`, 0)
	roots := rec.Roots()
	step := roots[0].Step
	list, ok := step.Out.(*object.List)
	if !ok {
		t.Fatalf("Out = %T, want *object.List", step.Out)
	}
	if len(list.Elements) != 3 {
		t.Fatalf("got %d elements, want 3", len(list.Elements))
	}
	if !step.SizeOK || step.Size != 3 {
		t.Errorf("Size, SizeOK = %d, %v, want 3, true", step.Size, step.SizeOK)
	}
}

func TestRecorderStepReportsErrorAsFailed(t *testing.T) {
	rec := record(t, `x = 1 / 0`, 0)
	roots := rec.Roots()
	if !roots[0].Step.Failed() {
		t.Error("Failed() = false, want true for a division-by-zero step")
	}
}

func TestRecorderStepLineNumber(t *testing.T) {
	rec := record(t, "x = 1\ny = 2\n", 0)
	roots := rec.Roots()
	if roots[0].Step.Line() != 1 {
		t.Errorf("roots[0] line = %d, want 1", roots[0].Step.Line())
	}
	if roots[1].Step.Line() != 2 {
		t.Errorf("roots[1] line = %d, want 2", roots[1].Step.Line())
	}
}

func TestStepLineOnNilStep(t *testing.T) {
	var s *Step
	if got := s.Line(); got != 0 {
		t.Errorf("Line() on nil Step = %d, want 0", got)
	}
}

func TestTraceNodeLabelOnFrame(t *testing.T) {
	n := &TraceNode{Frame: "double(...)"}
	if got := n.Label(); got != "double(...)" {
		t.Errorf("Label() = %q, want %q", got, "double(...)")
	}
}

func TestTraceNodeIterationsOnUnfoldedRow(t *testing.T) {
	n := &TraceNode{Frame: "double(...)"}
	laps, ok := n.Iterations()
	if ok || laps != 0 {
		t.Errorf("Iterations() = %d, %v, want 0, false for an unfolded row", laps, ok)
	}
}

func TestPushFrameDefendsBlankLabel(t *testing.T) {
	rec := NewRecorder(0)
	rec.PushFrame("")
	rec.PopFrame()
	// No Step() call ever adopts this frame (nothing else happened on
	// this Recorder), and the run isn't truncated, so it surfaces as an
	// ordinary root -- see TestRecorderUnclaimedFrameIsAnOrdinaryRootWhenNotTruncated.
	// What matters here is just that the blank-label defensive default
	// kicked in.
	roots := rec.Roots()
	if len(roots) != 1 || roots[0].Frame != "(frame)" {
		t.Errorf("roots = %+v, want one frame labelled \"(frame)\"", roots)
	}
}

// TestRecorderUnclaimedFrameIsAnOrdinaryRootWhenNotTruncated is a
// regression test for a real bug found while trying `crust develop`
// against an actual file: a program's whole logic almost always lives
// inside its store() entry point, and `crust develop` calls that via
// interp.CallNamed directly from Go code -- not from a traced
// *statement*, so nothing ever adopts the frame it opens. Before this
// fix, Roots()
// treated *any* leftover pending frame as evidence of an incomplete
// run and wrapped it in a misleading "(incomplete...)" row, even
// though the program ran to completion perfectly normally.
func TestRecorderUnclaimedFrameIsAnOrdinaryRootWhenNotTruncated(t *testing.T) {
	rec := NewRecorder(0)
	rec.PushFrame("store()")
	rec.PopFrame()
	if rec.Truncated() {
		t.Fatal("this recorder was never truncated")
	}
	roots := rec.Roots()
	if len(roots) != 1 || roots[0].Frame != "store()" {
		t.Errorf("roots = %+v, want one plain root frame \"store()\", not an incomplete wrapper", roots)
	}
}

func TestPopFrameOnEmptyStackIsANoOp(t *testing.T) {
	rec := NewRecorder(0)
	rec.PopFrame() // must not panic
	if len(rec.Roots()) != 0 {
		t.Errorf("Roots() = %+v, want empty", rec.Roots())
	}
}

func TestSummaryUntruncated(t *testing.T) {
	rec := record(t, "x = 1", 0)
	if strings.Contains(rec.Summary(), "capped") {
		t.Errorf("Summary() = %q, should not mention capped for an uncapped run", rec.Summary())
	}
	if !strings.Contains(rec.Summary(), "1 step") {
		t.Errorf("Summary() = %q, want it to mention 1 step", rec.Summary())
	}
}

func TestFoldLeavesMixedChildrenAlone(t *testing.T) {
	children := []*TraceNode{
		{Frame: "a"},
		{Step: &Step{}},
		{Frame: "b"},
	}
	got := fold(children)
	if len(got) != len(children) {
		t.Errorf("fold() = %+v, want the mixed slice returned unchanged", got)
	}
}

func TestRecorderIncompleteRunKeepsOrphanedFrames(t *testing.T) {
	// A normal cRust runtime error (unlike a Go panic) propagates via
	// ordinary return values, so every deferred PopFrame still fires
	// and the statement that opened a frame still gets to report --
	// there's nothing orphaned about it, just a step whose Out happens
	// to be an Error (see TestRecorderStepReportsErrorAsFailed). A
	// frame is only left truly pending when the step cap is hit while
	// it's still open: Step() becomes a permanent no-op once truncated,
	// so the step that would have adopted the frame never arrives.
	// maxSteps=1 guarantees the cap lands inside double's own frame,
	// before the call-site step ("y = double(5)") ever gets to report.
	rec := record(t, `
recipe double(x) {
    serve x * 2
}
y = double(5)
`, 1)
	if !rec.Truncated() {
		t.Fatal("expected the recording to be truncated")
	}
	roots := rec.Roots()
	last := roots[len(roots)-1]
	if !strings.Contains(last.Frame, "incomplete") {
		t.Fatalf("last root = %+v, want the synthetic incomplete-run row", last)
	}
}

func TestMergeAddsOneWrapperFrameWithOtherAsChildren(t *testing.T) {
	base := record(t, "a = 1\n", 0)
	other := record(t, "b = 2\nc = 3\n", 0)

	base.Merge("store_part2(...)", other)

	roots := base.Roots()
	if len(roots) != 2 {
		t.Fatalf("got %d roots, want 2 (the original root plus one merged wrapper)", len(roots))
	}
	wrapper := roots[1]
	if !wrapper.IsFrame() || wrapper.Frame != "store_part2(...)" {
		t.Fatalf("roots[1] = %+v, want a frame labeled store_part2(...)", wrapper)
	}
	if len(wrapper.Children) != 2 {
		t.Fatalf("wrapper has %d children, want other's 2 roots", len(wrapper.Children))
	}
	if wrapper.Children[0].Label() != "b = 2" || wrapper.Children[1].Label() != "c = 3" {
		t.Errorf("wrapper.Children = %+v, want other's own roots in order", wrapper.Children)
	}
}

func TestMergeSumsSteps(t *testing.T) {
	base := record(t, "a = 1\nb = 2\n", 0)
	other := record(t, "c = 3\n", 0)
	base.Merge("other", other)
	if base.Steps() != 3 {
		t.Errorf("Steps() = %d, want 3 (2 + 1)", base.Steps())
	}
}

func TestMergePropagatesTruncated(t *testing.T) {
	base := record(t, "a = 1\n", 0)
	truncated := record(t, `
recipe double(x) {
    serve x * 2
}
y = double(5)
`, 1)
	if !truncated.Truncated() {
		t.Fatal("expected the source recording to be truncated")
	}
	base.Merge("other", truncated)
	if !base.Truncated() {
		t.Error("expected Merge to propagate other's truncated flag onto base")
	}
}

func TestMergeOnUntruncatedBaseWithUntruncatedOtherStaysUntruncated(t *testing.T) {
	base := record(t, "a = 1\n", 0)
	other := record(t, "b = 2\n", 0)
	base.Merge("other", other)
	if base.Truncated() {
		t.Error("Merge should not mark base truncated when neither side was")
	}
}

func TestMergeMultipleTimes(t *testing.T) {
	base := NewRecorder(0)
	part1 := record(t, `deliver("one")`, 0)
	part2 := record(t, `deliver("two")`, 0)
	base.Merge("store_part1(...)", part1)
	base.Merge("store_part2(...)", part2)

	roots := base.Roots()
	if len(roots) != 2 {
		t.Fatalf("got %d roots, want 2 (one wrapper per merge)", len(roots))
	}
	if roots[0].Frame != "store_part1(...)" || roots[1].Frame != "store_part2(...)" {
		t.Errorf("roots = %+v, want the two merges in order", roots)
	}
}
