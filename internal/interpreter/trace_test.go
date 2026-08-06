package interpreter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Sintfoap/cRust/internal/ast"
	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/object"
	"github.com/Sintfoap/cRust/internal/parser"
	"github.com/Sintfoap/cRust/internal/trace"
)

// fakeTracer records every call it receives, in order, as a flat log —
// enough to assert both what each Step carried and how PushFrame/
// PopFrame nested around it, without needing a real tree builder
// (that's internal/debugger's job, tested separately).
type fakeTracer struct {
	log []string
	out []object.Object
}

func (f *fakeTracer) Step(e trace.StepEvent) {
	f.log = append(f.log, "step:"+e.Node.String())
	f.out = append(f.out, e.Out)
}
func (f *fakeTracer) PushFrame(label string) { f.log = append(f.log, "push:"+label) }
func (f *fakeTracer) PopFrame()              { f.log = append(f.log, "pop") }

func tracedEval(t *testing.T, input string) (*fakeTracer, object.Object) {
	t.Helper()
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parser errors for %q: %v", input, errs)
	}
	interp := New(&bytes.Buffer{}, strings.NewReader(""))
	ft := &fakeTracer{}
	interp.Trace = ft
	env := object.NewEnvironment()
	result := interp.Eval(program, env)
	return ft, result
}

func TestTraceReportsOneStepPerTopLevelStatement(t *testing.T) {
	ft, _ := tracedEval(t, `
x = 1
y = 2
`)
	want := []string{"step:x = 1", "step:y = 2"}
	if len(ft.log) != len(want) {
		t.Fatalf("log = %v, want %v", ft.log, want)
	}
	for i, w := range want {
		if ft.log[i] != w {
			t.Errorf("log[%d] = %q, want %q", i, ft.log[i], w)
		}
	}
}

func TestTraceStepOutIsTheAssignedValue(t *testing.T) {
	ft, _ := tracedEval(t, "x = 1 + 2")
	if len(ft.out) != 1 {
		t.Fatalf("got %d steps, want 1", len(ft.out))
	}
	wantInteger(t, ft.out[0], 3)
}

func TestTraceRecipeCallOpensAndClosesAFrame(t *testing.T) {
	ft, _ := tracedEval(t, `
recipe double(x) {
    serve x * 2
}
y = double(21)
`)
	// The `recipe double(x) {...}` declaration is its own step (an
	// AssignStatement binding double), then the call site is a step
	// whose frame wraps double's body -- "serve x * 2" -- as a nested
	// step of its own.
	want := []string{
		"step:recipe double(x) {\nserve (x * 2)\n}",
		"push:double(...)",
		"step:serve (x * 2)",
		"pop",
		"step:y = double(21)",
	}
	if len(ft.log) != len(want) {
		t.Fatalf("log = %v, want %v", ft.log, want)
	}
	for i, w := range want {
		if ft.log[i] != w {
			t.Errorf("log[%d] = %q, want %q", i, ft.log[i], w)
		}
	}
}

// TestTraceCallNamedUsesTheGivenLabel exercises the debugger-facing
// entry point cmd/crust's --store resolution goes through: unlike an
// ordinary CallExpression (which builds its own frame label from the
// call site's source text) or the plain Call export (a fixed generic
// "call(...)", meant for map()'s per-element callback), CallNamed has
// no source-level call site to read a name from at all, so its label
// comes entirely from the name argument -- this is what lets the
// debugger attribute a whole run to "store_part1(...)" instead of an
// anonymous frame indistinguishable from any other generic call.
func TestTraceCallNamedUsesTheGivenLabel(t *testing.T) {
	l := lexer.New(`recipe store_part1(x) { serve x * 2 }`)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parser errors: %v", errs)
	}
	interp := New(&bytes.Buffer{}, strings.NewReader(""))
	ft := &fakeTracer{}
	interp.Trace = ft
	env := object.NewEnvironment()
	interp.Eval(program, env)

	fn, ok := env.Get("store_part1")
	if !ok {
		t.Fatal("store_part1 was not bound in env")
	}
	ft.log = nil // only care about what CallNamed itself reports

	result := interp.CallNamed(fn, []object.Object{object.NewInteger(21)}, "store_part1")
	wantInteger(t, result, 42)

	want := []string{"push:store_part1(...)", "step:serve (x * 2)", "pop"}
	if len(ft.log) != len(want) {
		t.Fatalf("log = %v, want %v", ft.log, want)
	}
	for i, w := range want {
		if ft.log[i] != w {
			t.Errorf("log[%d] = %q, want %q", i, ft.log[i], w)
		}
	}
}

func TestTraceKneadForEachOpensOneFramePerLap(t *testing.T) {
	ft, _ := tracedEval(t, `
total = 0
knead n in [1, 2, 3] {
    total += n
}
`)
	want := []string{
		"step:total = 0",
		"push:knead n in ... lap 1",
		"step:total += n",
		"pop",
		"push:knead n in ... lap 2",
		"step:total += n",
		"pop",
		"push:knead n in ... lap 3",
		"step:total += n",
		"pop",
		// the knead statement itself is a step too, reported once it
		// finishes -- it wraps the lap frames above, the same way a
		// recipe call's own step wraps its body's frame.
		"step:knead n in [1, 2, 3] {\ntotal += n\n}",
	}
	if len(ft.log) != len(want) {
		t.Fatalf("log = %v, want %v", ft.log, want)
	}
	for i, w := range want {
		if ft.log[i] != w {
			t.Errorf("log[%d] = %q, want %q", i, ft.log[i], w)
		}
	}
}

func TestTraceBakeLoopOpensOneFramePerLap(t *testing.T) {
	ft, _ := tracedEval(t, `
n = 0
bake (n < 2) {
    n += 1
}
`)
	want := []string{
		"step:n = 0",
		"push:bake (...) lap 1",
		"step:n += 1",
		"pop",
		"push:bake (...) lap 2",
		"step:n += 1",
		"pop",
		// bake itself is a step too, same as knead above.
		"step:bake ((n < 2)) {\nn += 1\n}",
	}
	if len(ft.log) != len(want) {
		t.Fatalf("log = %v, want %v", ft.log, want)
	}
	for i, w := range want {
		if ft.log[i] != w {
			t.Errorf("log[%d] = %q, want %q", i, ft.log[i], w)
		}
	}
}

func TestTraceCountedLoopOpensOneFramePerLap(t *testing.T) {
	ft, _ := tracedEval(t, `
knead (i = 0; i < 2; i++) {
    x = i
}
`)
	var pushes int
	for _, l := range ft.log {
		if strings.HasPrefix(l, "push:knead (...) lap") {
			pushes++
		}
	}
	if pushes != 2 {
		t.Errorf("got %d loop-lap frames, want 2; log = %v", pushes, ft.log)
	}
}

func TestTraceNestedRecipeCallsNestFrames(t *testing.T) {
	ft, _ := tracedEval(t, `
recipe inner() {
    serve 1
}
recipe outer() {
    serve inner()
}
outer()
`)
	// Two pushes before the first pop that closes inner's frame.
	pushIdx := -1
	for i, l := range ft.log {
		if l == "push:inner(...)" {
			pushIdx = i
		}
	}
	if pushIdx == -1 {
		t.Fatalf("inner(...) frame never opened; log = %v", ft.log)
	}
	// The frame opened just before it must be outer's.
	found := false
	for i := 0; i < pushIdx; i++ {
		if ft.log[i] == "push:outer(...)" {
			found = true
		}
	}
	if !found {
		t.Errorf("outer(...) frame not opened before inner(...); log = %v", ft.log)
	}
}

func TestTraceNilByDefault(t *testing.T) {
	interp := New(&bytes.Buffer{}, strings.NewReader(""))
	if interp.Trace != nil {
		t.Error("Trace should be nil unless explicitly set")
	}
}

func TestTraceUnusedWhenNil(t *testing.T) {
	// No panic, no special behavior -- an ordinary Eval with Trace nil.
	wantInteger(t, testEval(t, "1 + 2"), 3)
}

const benchProgram = `
recipe fib(n) {
    order (n < 2) {
        serve n
    }
    serve fib(n - 1) + fib(n - 2)
}
total = 0
knead i in 0.<15 {
    total += fib(i)
}
`

func newBenchInterpreter(t *testing.B) (*Interpreter, *ast.Program, *object.Environment) {
	t.Helper()
	l := lexer.New(benchProgram)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parser errors: %v", errs)
	}
	return New(&bytes.Buffer{}, strings.NewReader("")), program, object.NewEnvironment()
}

// BenchmarkTracedVsUntraced pins the claim evalTracedStatement/
// evalFramed's doc comments make: a nil Tracer costs the one nil
// check per statement/frame and nothing more. Run with
// `go test -bench Traced -benchmem ./internal/interpreter` to see the
// two numbers side by side; there's no automated threshold here (that
// would be a flaky test keyed to whatever machine runs it), just a
// benchmark a human can read.
func BenchmarkUntraced(b *testing.B) {
	interp, program, env := newBenchInterpreter(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		interp.Eval(program, env)
	}
}

func BenchmarkTraced(b *testing.B) {
	interp, program, env := newBenchInterpreter(b)
	interp.Trace = &fakeTracer{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		interp.Eval(program, env)
	}
}
