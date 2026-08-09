// Package interpreter tree-walks an internal/ast tree and produces
// internal/object values — cRust's Eval. See ARCHITECTURE.md's Phase 4
// section for the design this implements: one recursive Eval switching
// on the AST node's Go type, no separate compile step.
package interpreter

import (
	"fmt"
	"io"

	"github.com/Sintfoap/cRust/internal/ast"
	"github.com/Sintfoap/cRust/internal/builtins"
	"github.com/Sintfoap/cRust/internal/object"
	"github.com/Sintfoap/cRust/internal/token"
	"github.com/Sintfoap/cRust/internal/trace"
)

// Interpreter holds what Eval needs beyond the AST node and
// Environment it's given: the builtin table. A struct (with Eval as a
// method) rather than a package-level Eval function specifically so
// Builtins isn't global mutable state — each Interpreter (each `crust
// run`, each REPL session, each test) gets its own, bound to its own
// output writer.
//
// Trace is nil for an ordinary run — internal/trace's Tracer is
// checked once per statement (evalBlockStatement) and once per call/
// loop-lap frame (evalFramed), and every one of those checks costs
// nothing when it's nil; see BenchmarkTracedVsUntraced. `crust develop`
// is the one caller that sets it, to a *debugger.Recorder.
//
// BaseDir is the directory a `delivery "path.crust"` statement resolves
// a relative Path against (evalDeliveryStatement, delivery.go) — the
// directory of whichever file is actually running, not necessarily the
// process's own working directory. Left at its zero value ("") by New,
// which os.ReadFile/filepath.Join already treat as "resolve against the
// current working directory" with no special-casing needed — exactly
// right for the REPL, which has no backing file of its own. Every
// caller that *does* have a real path (runner.Run, `crust develop`) is
// expected to set it right after New returns.
type Interpreter struct {
	Builtins map[string]*object.Builtin
	Trace    trace.Tracer
	BaseDir  string

	// delivered tracks every distinct file (by absolute path) a
	// DeliveryStatement has already evaluated, so importing the same
	// helper file from two different places in a program (or from two
	// files that both deliver a shared third one) runs its top level
	// exactly once — same reasoning Go's own import/Python's module
	// cache both apply, and the mechanism that also makes a circular
	// delivery terminate instead of recursing forever: a file is marked
	// delivered *before* its own top level runs (evalDeliveryStatement),
	// so a cycle's second, inward delivery attempt always finds itself
	// already marked and simply no-ops rather than re-entering.
	delivered map[string]bool
}

// New returns an Interpreter whose `deliver` builtin writes to output,
// whose `unbox` builtin (with no argument) reads from stdin, and whose
// `map` builtin calls back into this same Interpreter's own Call to
// invoke user-defined functions — i's Builtins field is set after i
// itself exists specifically so the i.Call method value passed to
// builtins.New already refers to the right instance; Call only reads
// i.Builtins at actual call time (long after this constructor
// returns), never during construction, so the field being briefly
// unset here is never observed.
func New(output io.Writer, stdin io.Reader) *Interpreter {
	i := &Interpreter{delivered: make(map[string]bool)}
	i.Builtins = builtins.New(output, stdin, i.Call)
	return i
}

// Call invokes fn (a *object.Function or *object.Builtin) with args,
// the same way evaluating a CallExpression would — exported so callers
// like the `map` builtin can invoke an already-resolved function
// directly, without a source-level CallExpression to Eval. There's no
// call-site token in this case, so any resulting Error's position is
// left unset (0, 0). The generic "call(...)" frame label is deliberate
// here: `map` invokes its callback once per element, and a distinct
// label per element would just be noise in the debugger's KPI/stepper
// — every caller that *does* have a real name to attach (cmd/crust's
// entry-point resolution, SPEC.md §9) should use CallNamed instead, not
// this one. The nil passed for applyFunction's calleeExpr means an
// Error's stack trace names this frame "call(...)" too — there's no
// source-level call expression here for it to read a real name from
// either.
func (i *Interpreter) Call(fn object.Object, args []object.Object) object.Object {
	return i.applyFunction(token.Token{}, "call(...)", nil, fn, args)
}

// CallNamed is Call with a real frame label instead of the generic
// "call(...)" — for the one other case an already-resolved function
// gets invoked without a source-level CallExpression: cmd/crust
// resolving and running a store/store_<name> entry point (SPEC.md §9).
// Without this, that invocation went through the same Call every
// map() callback does, so the debugger's KPI/stepper attributed the
// entire run to an anonymous "call(...)" frame instead of the actual
// recipe name — indistinguishable from any other generic call, and
// exactly the opposite of what a single-entry-point program's KPI
// breakdown should show. name should be the bare recipe name (e.g.
// "store_part1"); the "(...)" suffix matches the label an ordinary
// CallExpression's own evalCallExpression already builds.
func (i *Interpreter) CallNamed(fn object.Object, args []object.Object, name string) object.Object {
	return i.applyFunction(token.Token{}, name+"(...)", nil, fn, args)
}

// Eval evaluates node in env and returns the resulting Object. It never
// returns a Go nil — every path produces a real Object, most commonly
// object.NULL for constructs with no meaningful value of their own
// (order/knead/bake, burnt/flip as bare statements aside — those
// return the Break/Continue signal objects instead, not NULL).
func (i *Interpreter) Eval(node ast.Node, env *object.Environment) object.Object {
	switch node := node.(type) {

	// Program & statements.
	case *ast.Program:
		return i.evalProgram(node, env)
	case *ast.BlockStatement:
		return i.evalBlockStatement(node, env)
	case *ast.ExpressionStatement:
		return i.Eval(node.Expression, env)
	case *ast.AssignStatement:
		return i.evalAssignStatement(node, env)
	case *ast.UnpackAssignStatement:
		return i.evalUnpackAssignStatement(node, env)
	case *ast.IncDecStatement:
		return i.evalIncDecStatement(node, env)
	case *ast.ReturnStatement:
		return i.evalReturnStatement(node, env)
	case *ast.BurntStatement:
		return object.BREAK
	case *ast.FlipStatement:
		return object.CONTINUE
	case *ast.IfStatement:
		return i.evalIfStatement(node, env)
	case *ast.CountedLoop:
		return i.evalCountedLoop(node, env)
	case *ast.ForEachLoop:
		return i.evalForEachLoop(node, env)
	case *ast.BakeStatement:
		return i.evalBakeStatement(node, env)
	case *ast.DeliveryStatement:
		return i.evalDeliveryStatement(node, env)

	// Literals.
	case *ast.IntegerLiteral:
		return object.NewInteger(node.Value)
	case *ast.FloatLiteral:
		return &object.Float{Value: node.Value}
	case *ast.StringLiteral:
		return &object.String{Value: node.Value}
	case *ast.BooleanLiteral:
		return object.NativeBoolToBooleanObject(node.Value)
	case *ast.NilLiteral:
		return object.NULL

	// Expressions.
	case *ast.Identifier:
		return i.evalIdentifier(node, env)
	case *ast.PrefixExpression:
		return i.evalPrefixExpression(node, env)
	case *ast.InfixExpression:
		return i.evalInfixExpression(node, env)
	case *ast.RangeExpression:
		return i.evalRangeExpression(node, env)
	case *ast.TernaryExpression:
		return i.evalTernaryExpression(node, env)
	case *ast.ElvisExpression:
		return i.evalElvisExpression(node, env)
	case *ast.CallExpression:
		return i.evalCallExpression(node, env)
	case *ast.IndexExpression:
		return i.evalIndexExpression(node, env)
	case *ast.FunctionLiteral:
		return i.evalFunctionLiteral(node, env)
	case *ast.ListLiteral:
		return i.evalListLiteral(node, env)
	case *ast.MapLiteral:
		return i.evalMapLiteral(node, env)
	case *ast.SetLiteral:
		return i.evalSetLiteral(node, env)
	case *ast.TupleLiteral:
		return i.evalTupleLiteral(node, env)
	}

	return newError(token.Token{}, "eval: unsupported node type %T", node)
}

// newError builds a runtime Error positioned at tok — the standard way
// every eval* function reports a problem, from a missing variable to a
// type mismatch.
func newError(tok token.Token, format string, args ...any) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, args...), Line: tok.Line, Col: tok.Col}
}

func isError(obj object.Object) bool {
	if obj == nil {
		return false
	}
	return obj.Type() == object.ERROR_OBJ
}

// unwrapReturnValue is what a function call does to the result of
// evaluating its body — turns a bubbled-up ReturnValue back into the
// plain value inside, or passes through anything else (NULL for a
// recipe that ran off the end without `serve`, an Error) unchanged.
func unwrapReturnValue(obj object.Object) object.Object {
	if rv, ok := obj.(*object.ReturnValue); ok {
		return rv.Value
	}
	return obj
}

// numericValue reports v's value as a float64 if v is an Integer or
// Float, for the arithmetic/comparison code that treats them as one
// "number" category (SPEC.md §6).
func numericValue(obj object.Object) (float64, bool) {
	switch o := obj.(type) {
	case *object.Integer:
		return float64(o.Value), true
	case *object.Float:
		return o.Value, true
	default:
		return 0, false
	}
}

// SPEC.md §6's `==`/`!=` rule (value equality across Lists/Tuples/
// Maps/Sets/Grids, cross-category always false) lives in
// object.Equal — shared with any builtin that needs the exact same
// notion of "equal" (e.g. contains()) rather than duplicated here.
