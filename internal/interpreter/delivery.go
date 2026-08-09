// `delivery "path.crust"` (SPEC.md §10) — cRust's module system.
// Deliberately the simplest thing that could work for AoC-shaped
// programs: no namespacing, no exports list, no separate module
// value — a delivery statement just lexes, parses, and evaluates
// another file's top level directly into the *current* Environment,
// the same as if that file's text had been pasted in at the delivery
// statement's own location. Every name it binds (a recipe, a
// top-level variable) becomes an ordinary name in the importing
// file's scope from that point on, resolved by the exact same
// Environment.Set rule (SPEC.md §3) as everything else — a later
// definition with the same name simply wins, no special collision
// handling needed because there's no new mechanism here beyond
// "run more statements against this Environment."
package interpreter

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Sintfoap/cRust/internal/ast"
	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/object"
	"github.com/Sintfoap/cRust/internal/parser"
)

// evalDeliveryStatement resolves ds.Path against i.BaseDir, reads and
// parses the target file, and evaluates its top-level statements into
// env — env is whatever scope the delivery statement itself is
// running in, not a fresh one, since a delivered recipe needs to end
// up directly visible to the importing code, not tucked away in a
// sub-scope nothing else can see.
//
// Already-delivered files (i.delivered, keyed by absolute path) are a
// silent no-op the second time — both a plain optimization (a shared
// helper file delivered from two different places in a program
// shouldn't re-run its own top level twice) and what keeps a circular
// delivery (A delivers B, B delivers A) from recursing forever: A is
// marked delivered *before* its own top level starts running, so by
// the time B's own delivery of A is reached, A already shows as
// delivered and B's attempt is simply skipped rather than re-entering
// A's evaluation.
//
// i.BaseDir is swapped to the delivered file's own directory for the
// duration of evaluating its top level, then restored — so a relative
// delivery *inside* the delivered file resolves against wherever that
// file actually lives, not wherever the original top-level file was,
// the same relative-to-the-current-file rule most module systems use.
func (i *Interpreter) evalDeliveryStatement(ds *ast.DeliveryStatement, env *object.Environment) object.Object {
	resolved := ds.Path
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(i.BaseDir, resolved)
	}

	abs, err := filepath.Abs(resolved)
	if err != nil {
		abs = resolved
	}
	if i.delivered[abs] {
		return object.NULL
	}
	i.delivered[abs] = true

	src, err := os.ReadFile(resolved)
	if err != nil {
		return newError(ds.Pos(), "delivery %q: %s", ds.Path, err)
	}

	l := lexer.New(string(src))
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return newError(ds.Pos(), "delivery %q: %d parse error(s): %s", ds.Path, len(errs), strings.Join(errs, "; "))
	}

	prevBaseDir := i.BaseDir
	i.BaseDir = filepath.Dir(resolved)
	defer func() { i.BaseDir = prevBaseDir }()

	result := i.Eval(program, env)
	if errObj, ok := result.(*object.Error); ok {
		return newError(ds.Pos(), "delivery %q: %d:%d: %s", ds.Path, errObj.Line, errObj.Col, errObj.Message)
	}
	return object.NULL
}
