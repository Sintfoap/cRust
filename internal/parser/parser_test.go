package parser

import (
	"fmt"
	"testing"

	"github.com/Sintfoap/cRust/internal/ast"
	"github.com/Sintfoap/cRust/internal/lexer"
)

// parseProgram lexes and parses input, failing the test immediately if
// any parse errors were recorded.
func parseProgram(t *testing.T, input string) *ast.Program {
	t.Helper()
	l := lexer.New(input)
	p := New(l)
	program := p.ParseProgram()
	checkParserErrors(t, p)
	return program
}

func checkParserErrors(t *testing.T, p *Parser) {
	t.Helper()
	errs := p.Errors()
	if len(errs) == 0 {
		return
	}
	t.Errorf("parser had %d error(s)", len(errs))
	for _, e := range errs {
		t.Errorf("parser error: %s", e)
	}
}

func singleStatement(t *testing.T, program *ast.Program) ast.Statement {
	t.Helper()
	if len(program.Statements) != 1 {
		t.Fatalf("program has %d statements, want 1: %+v", len(program.Statements), program.Statements)
	}
	return program.Statements[0]
}

func exprStmtExpr(t *testing.T, stmt ast.Statement) ast.Expression {
	t.Helper()
	es, ok := stmt.(*ast.ExpressionStatement)
	if !ok {
		t.Fatalf("statement is %T, want *ast.ExpressionStatement", stmt)
	}
	return es.Expression
}

func testIdentifier(t *testing.T, exp ast.Expression, want string) {
	t.Helper()
	ident, ok := exp.(*ast.Identifier)
	if !ok {
		t.Fatalf("expression is %T, want *ast.Identifier", exp)
	}
	if ident.Value != want {
		t.Errorf("identifier.Value = %q, want %q", ident.Value, want)
	}
}

func testIntegerLiteral(t *testing.T, exp ast.Expression, want int64) {
	t.Helper()
	il, ok := exp.(*ast.IntegerLiteral)
	if !ok {
		t.Fatalf("expression is %T, want *ast.IntegerLiteral", exp)
	}
	if il.Value != want {
		t.Errorf("IntegerLiteral.Value = %d, want %d", il.Value, want)
	}
}

// --- Assignment statements ---------------------------------------------

func TestAssignStatements(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
		wantOp   string
	}{
		{"x = 5\n", "x", "="},
		{"x += 5\n", "x", "+="},
		{"x -= 5\n", "x", "-="},
		{"x *= 5\n", "x", "*="},
		{"x /= 5\n", "x", "/="},
		{"x %= 5\n", "x", "%="},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program := parseProgram(t, tt.input)
			stmt := singleStatement(t, program)
			as, ok := stmt.(*ast.AssignStatement)
			if !ok {
				t.Fatalf("statement is %T, want *ast.AssignStatement", stmt)
			}
			testIdentifier(t, as.Target, tt.wantName)
			if as.Operator != tt.wantOp {
				t.Errorf("Operator = %q, want %q", as.Operator, tt.wantOp)
			}
			testIntegerLiteral(t, as.Value, 5)
		})
	}
}

func TestAssignToIndexExpression(t *testing.T) {
	program := parseProgram(t, "arr[0] = 5\n")
	stmt := singleStatement(t, program)
	as, ok := stmt.(*ast.AssignStatement)
	if !ok {
		t.Fatalf("statement is %T, want *ast.AssignStatement", stmt)
	}
	if _, ok := as.Target.(*ast.IndexExpression); !ok {
		t.Fatalf("Target is %T, want *ast.IndexExpression", as.Target)
	}
}

func TestInvalidAssignTargetIsError(t *testing.T) {
	l := lexer.New("1 + 2 = 3\n")
	p := New(l)
	p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("expected a parse error for an invalid assignment target, got none")
	}
}

// --- Unpack assignment ---------------------------------------------------

func TestUnpackAssignStatement(t *testing.T) {
	program := parseProgram(t, "a, b, c = xs\n")
	stmt := singleStatement(t, program)
	uas, ok := stmt.(*ast.UnpackAssignStatement)
	if !ok {
		t.Fatalf("statement is %T, want *ast.UnpackAssignStatement", stmt)
	}
	if len(uas.Targets) != 3 {
		t.Fatalf("got %d targets, want 3", len(uas.Targets))
	}
	wantNames := []string{"a", "b", "c"}
	for i, want := range wantNames {
		if uas.Targets[i].Value != want {
			t.Errorf("Targets[%d] = %q, want %q", i, uas.Targets[i].Value, want)
		}
	}
	testIdentifier(t, uas.Value, "xs")
}

// --- Inc/Dec statements ---------------------------------------------------

func TestIncDecStatements(t *testing.T) {
	tests := []struct {
		input  string
		wantOp string
	}{
		{"x++\n", "++"},
		{"x--\n", "--"},
		{"++x\n", "++"},
		{"--x\n", "--"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program := parseProgram(t, tt.input)
			stmt := singleStatement(t, program)
			ids, ok := stmt.(*ast.IncDecStatement)
			if !ok {
				t.Fatalf("statement is %T, want *ast.IncDecStatement", stmt)
			}
			if ids.Operator != tt.wantOp {
				t.Errorf("Operator = %q, want %q", ids.Operator, tt.wantOp)
			}
			testIdentifier(t, ids.Target, "x")
		})
	}
}

func TestIncDecOnIndexExpression(t *testing.T) {
	program := parseProgram(t, "arr[0]++\n")
	stmt := singleStatement(t, program)
	ids, ok := stmt.(*ast.IncDecStatement)
	if !ok {
		t.Fatalf("statement is %T, want *ast.IncDecStatement", stmt)
	}
	if _, ok := ids.Target.(*ast.IndexExpression); !ok {
		t.Fatalf("Target is %T, want *ast.IndexExpression", ids.Target)
	}
}

// --- Return / burnt / flip -------------------------------------------------

func TestReturnStatements(t *testing.T) {
	program := parseProgram(t, "serve 5\n")
	stmt := singleStatement(t, program)
	rs, ok := stmt.(*ast.ReturnStatement)
	if !ok {
		t.Fatalf("statement is %T, want *ast.ReturnStatement", stmt)
	}
	testIntegerLiteral(t, rs.ReturnValue, 5)
}

func TestBareReturnStatement(t *testing.T) {
	program := parseProgram(t, "serve\n")
	stmt := singleStatement(t, program)
	rs, ok := stmt.(*ast.ReturnStatement)
	if !ok {
		t.Fatalf("statement is %T, want *ast.ReturnStatement", stmt)
	}
	if rs.ReturnValue != nil {
		t.Errorf("ReturnValue = %v, want nil", rs.ReturnValue)
	}
}

func TestBareReturnBeforeBrace(t *testing.T) {
	program := parseProgram(t, "recipe f() { serve }\n")
	stmt := singleStatement(t, program)
	fn := stmt.(*ast.ExpressionStatement).Expression.(*ast.FunctionLiteral)
	rs := fn.Body.Statements[0].(*ast.ReturnStatement)
	if rs.ReturnValue != nil {
		t.Errorf("ReturnValue = %v, want nil", rs.ReturnValue)
	}
}

func TestBurntAndFlipStatements(t *testing.T) {
	program := parseProgram(t, "burnt\nflip\n")
	if len(program.Statements) != 2 {
		t.Fatalf("got %d statements, want 2", len(program.Statements))
	}
	if _, ok := program.Statements[0].(*ast.BurntStatement); !ok {
		t.Errorf("Statements[0] is %T, want *ast.BurntStatement", program.Statements[0])
	}
	if _, ok := program.Statements[1].(*ast.FlipStatement); !ok {
		t.Errorf("Statements[1] is %T, want *ast.FlipStatement", program.Statements[1])
	}
}

// --- Literals ---------------------------------------------------------------

func TestLiteralExpressions(t *testing.T) {
	tests := []struct {
		input string
		want  string // String() of the parsed expression
	}{
		{"5\n", "5"},
		{"3.14\n", "3.14"},
		{`"hi"` + "\n", `"hi"`},
		{"stuffed\n", "stuffed"},
		{"thin\n", "thin"},
		{"nobox\n", "nobox"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program := parseProgram(t, tt.input)
			stmt := singleStatement(t, program)
			exp := exprStmtExpr(t, stmt)
			if got := exp.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBooleanLiteralValues(t *testing.T) {
	program := parseProgram(t, "stuffed\nthin\n")
	if len(program.Statements) != 2 {
		t.Fatalf("got %d statements, want 2", len(program.Statements))
	}
	b0 := exprStmtExpr(t, program.Statements[0]).(*ast.BooleanLiteral)
	if b0.Value != true {
		t.Errorf("stuffed parsed as Value=%v, want true", b0.Value)
	}
	b1 := exprStmtExpr(t, program.Statements[1]).(*ast.BooleanLiteral)
	if b1.Value != false {
		t.Errorf("thin parsed as Value=%v, want false", b1.Value)
	}
}

// --- Prefix expressions ------------------------------------------------------

func TestPrefixExpressions(t *testing.T) {
	tests := []struct {
		input string
		op    string
	}{
		{"-15\n", "-"},
		{"hold stuffed\n", "hold"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program := parseProgram(t, tt.input)
			stmt := singleStatement(t, program)
			pe, ok := exprStmtExpr(t, stmt).(*ast.PrefixExpression)
			if !ok {
				t.Fatalf("expression is %T, want *ast.PrefixExpression", exprStmtExpr(t, stmt))
			}
			if pe.Operator != tt.op {
				t.Errorf("Operator = %q, want %q", pe.Operator, tt.op)
			}
		})
	}
}

// --- Operator precedence -----------------------------------------------------

func TestOperatorPrecedence(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"-a * b\n", "((- a) * b)\n"},
		{"a + b + c\n", "((a + b) + c)\n"},
		{"a + b - c\n", "((a + b) - c)\n"},
		{"a * b * c\n", "((a * b) * c)\n"},
		{"a * b / c\n", "((a * b) / c)\n"},
		{"a + b * c\n", "(a + (b * c))\n"},
		{"a + b * c + d / e - f\n", "(((a + (b * c)) + (d / e)) - f)\n"},
		{"5 > 4 == 3 < 4\n", "((5 > 4) == (3 < 4))\n"},
		{"5 < 4 != 3 > 4\n", "((5 < 4) != (3 > 4))\n"},
		{"3 + 4 * 5 == 3 * 1 + 4 * 5\n", "((3 + (4 * 5)) == ((3 * 1) + (4 * 5)))\n"},
		{"(a + b) * c\n", "((a + b) * c)\n"},
		{"a with b or c\n", "((a with b) or c)\n"},
		{"a or b with c\n", "(a or (b with c))\n"},
		{"a with b == c\n", "(a with (b == c))\n"},
		{"a + b == c with d\n", "(((a + b) == c) with d)\n"},
		{"a() + b\n", "(a() + b)\n"},
		{"a[0] + b\n", "((a[0]) + b)\n"},
		{"a ?: b ?: c\n", "(a ?: (b ?: c))\n"},
		{"a == b ?: c\n", "((a == b) ?: c)\n"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program := parseProgram(t, tt.input)
			if got := program.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInfixExpressionOperators(t *testing.T) {
	ops := []string{"+", "-", "*", "/", "%", "==", "!=", "<", ">", "<=", ">=", "with", "or"}
	for _, op := range ops {
		input := fmt.Sprintf("a %s b\n", op)
		t.Run(input, func(t *testing.T) {
			program := parseProgram(t, input)
			stmt := singleStatement(t, program)
			ie, ok := exprStmtExpr(t, stmt).(*ast.InfixExpression)
			if !ok {
				t.Fatalf("expression is %T, want *ast.InfixExpression", exprStmtExpr(t, stmt))
			}
			if ie.Operator != op {
				t.Errorf("Operator = %q, want %q", ie.Operator, op)
			}
			testIdentifier(t, ie.Left, "a")
			testIdentifier(t, ie.Right, "b")
		})
	}
}

// --- Ranges ---------------------------------------------------------------

func TestRangeExpressions(t *testing.T) {
	tests := []struct {
		input     string
		inclusive bool
	}{
		{"1..5\n", true},
		{"1.<5\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program := parseProgram(t, tt.input)
			stmt := singleStatement(t, program)
			re, ok := exprStmtExpr(t, stmt).(*ast.RangeExpression)
			if !ok {
				t.Fatalf("expression is %T, want *ast.RangeExpression", exprStmtExpr(t, stmt))
			}
			if re.Inclusive != tt.inclusive {
				t.Errorf("Inclusive = %v, want %v", re.Inclusive, tt.inclusive)
			}
			testIntegerLiteral(t, re.Start, 1)
			testIntegerLiteral(t, re.End, 5)
		})
	}
}

func TestChainedRangeIsError(t *testing.T) {
	l := lexer.New("1..5..10\n")
	p := New(l)
	p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("expected a parse error for a chained range, got none")
	}
}

// --- Ternary and Elvis -------------------------------------------------------

func TestTernaryExpression(t *testing.T) {
	program := parseProgram(t, "cond (| a |) b\n")
	stmt := singleStatement(t, program)
	te, ok := exprStmtExpr(t, stmt).(*ast.TernaryExpression)
	if !ok {
		t.Fatalf("expression is %T, want *ast.TernaryExpression", exprStmtExpr(t, stmt))
	}
	testIdentifier(t, te.Cond, "cond")
	testIdentifier(t, te.Then, "a")
	testIdentifier(t, te.Else, "b")
}

func TestChainedTernaryElse(t *testing.T) {
	program := parseProgram(t, "a (| 1 |) b (| 2 |) 3\n")
	stmt := singleStatement(t, program)
	outer := exprStmtExpr(t, stmt).(*ast.TernaryExpression)
	testIdentifier(t, outer.Cond, "a")
	testIntegerLiteral(t, outer.Then, 1)
	inner, ok := outer.Else.(*ast.TernaryExpression)
	if !ok {
		t.Fatalf("Else is %T, want *ast.TernaryExpression (chained)", outer.Else)
	}
	testIdentifier(t, inner.Cond, "b")
	testIntegerLiteral(t, inner.Then, 2)
	testIntegerLiteral(t, inner.Else, 3)
}

func TestElvisExpression(t *testing.T) {
	program := parseProgram(t, "a ?: b\n")
	stmt := singleStatement(t, program)
	ee, ok := exprStmtExpr(t, stmt).(*ast.ElvisExpression)
	if !ok {
		t.Fatalf("expression is %T, want *ast.ElvisExpression", exprStmtExpr(t, stmt))
	}
	testIdentifier(t, ee.Left, "a")
	testIdentifier(t, ee.Right, "b")
}

func TestElvisIsRightAssociative(t *testing.T) {
	program := parseProgram(t, "a ?: b ?: c\n")
	stmt := singleStatement(t, program)
	outer := exprStmtExpr(t, stmt).(*ast.ElvisExpression)
	testIdentifier(t, outer.Left, "a")
	inner, ok := outer.Right.(*ast.ElvisExpression)
	if !ok {
		t.Fatalf("Right is %T, want *ast.ElvisExpression (right-associative chain)", outer.Right)
	}
	testIdentifier(t, inner.Left, "b")
	testIdentifier(t, inner.Right, "c")
}

// --- Collections --------------------------------------------------------------

func TestListLiteral(t *testing.T) {
	program := parseProgram(t, "[1, 2, 3]\n")
	stmt := singleStatement(t, program)
	ll, ok := exprStmtExpr(t, stmt).(*ast.ListLiteral)
	if !ok {
		t.Fatalf("expression is %T, want *ast.ListLiteral", exprStmtExpr(t, stmt))
	}
	if len(ll.Elements) != 3 {
		t.Fatalf("got %d elements, want 3", len(ll.Elements))
	}
}

func TestEmptyListLiteral(t *testing.T) {
	program := parseProgram(t, "[]\n")
	stmt := singleStatement(t, program)
	ll := exprStmtExpr(t, stmt).(*ast.ListLiteral)
	if len(ll.Elements) != 0 {
		t.Fatalf("got %d elements, want 0", len(ll.Elements))
	}
}

// A bare '{' at statement start is always parsed as a block, never a
// map literal (see parseStatement) — so these use an assignment to put
// the '{' in expression position instead.

func TestMapLiteral(t *testing.T) {
	program := parseProgram(t, `m = {"a": 1, "b": 2}`+"\n")
	stmt := singleStatement(t, program)
	as := stmt.(*ast.AssignStatement)
	ml, ok := as.Value.(*ast.MapLiteral)
	if !ok {
		t.Fatalf("expression is %T, want *ast.MapLiteral", as.Value)
	}
	if len(ml.Pairs) != 2 {
		t.Fatalf("got %d pairs, want 2", len(ml.Pairs))
	}
}

func TestEmptyMapLiteral(t *testing.T) {
	program := parseProgram(t, "m = {}\n")
	stmt := singleStatement(t, program)
	as := stmt.(*ast.AssignStatement)
	ml := as.Value.(*ast.MapLiteral)
	if len(ml.Pairs) != 0 {
		t.Fatalf("got %d pairs, want 0", len(ml.Pairs))
	}
}

func TestSetLiteral(t *testing.T) {
	program := parseProgram(t, "toppings{1, 2, 3}\n")
	stmt := singleStatement(t, program)
	sl, ok := exprStmtExpr(t, stmt).(*ast.SetLiteral)
	if !ok {
		t.Fatalf("expression is %T, want *ast.SetLiteral", exprStmtExpr(t, stmt))
	}
	if len(sl.Elements) != 3 {
		t.Fatalf("got %d elements, want 3", len(sl.Elements))
	}
}

// --- Functions and calls --------------------------------------------------

func TestNamedFunctionLiteral(t *testing.T) {
	program := parseProgram(t, "recipe add(x, y) { serve x + y }\n")
	stmt := singleStatement(t, program)
	fn := exprStmtExpr(t, stmt).(*ast.FunctionLiteral)
	if fn.Name == nil || fn.Name.Value != "add" {
		t.Fatalf("Name = %v, want \"add\"", fn.Name)
	}
	if len(fn.Parameters) != 2 || fn.Parameters[0].Value != "x" || fn.Parameters[1].Value != "y" {
		t.Fatalf("Parameters = %+v, want [x y]", fn.Parameters)
	}
	if len(fn.Body.Statements) != 1 {
		t.Fatalf("Body has %d statements, want 1", len(fn.Body.Statements))
	}
}

func TestAnonymousFunctionLiteral(t *testing.T) {
	program := parseProgram(t, "apply = recipe(x) { serve x * 2 }\n")
	stmt := singleStatement(t, program)
	as := stmt.(*ast.AssignStatement)
	fn := as.Value.(*ast.FunctionLiteral)
	if fn.Name != nil {
		t.Errorf("Name = %v, want nil (anonymous)", fn.Name)
	}
}

func TestFunctionLiteralNoParameters(t *testing.T) {
	program := parseProgram(t, "recipe f() { serve 1 }\n")
	stmt := singleStatement(t, program)
	fn := exprStmtExpr(t, stmt).(*ast.FunctionLiteral)
	if len(fn.Parameters) != 0 {
		t.Fatalf("got %d parameters, want 0", len(fn.Parameters))
	}
}

func TestCallExpression(t *testing.T) {
	program := parseProgram(t, "add(1, 2 * 3, 4 + 5)\n")
	stmt := singleStatement(t, program)
	ce, ok := exprStmtExpr(t, stmt).(*ast.CallExpression)
	if !ok {
		t.Fatalf("expression is %T, want *ast.CallExpression", exprStmtExpr(t, stmt))
	}
	testIdentifier(t, ce.Function, "add")
	if len(ce.Arguments) != 3 {
		t.Fatalf("got %d arguments, want 3", len(ce.Arguments))
	}
	testIntegerLiteral(t, ce.Arguments[0], 1)
}

func TestIndexExpression(t *testing.T) {
	program := parseProgram(t, "arr[1 + 1]\n")
	stmt := singleStatement(t, program)
	ie, ok := exprStmtExpr(t, stmt).(*ast.IndexExpression)
	if !ok {
		t.Fatalf("expression is %T, want *ast.IndexExpression", exprStmtExpr(t, stmt))
	}
	testIdentifier(t, ie.Left, "arr")
	if _, ok := ie.Index.(*ast.InfixExpression); !ok {
		t.Fatalf("Index is %T, want *ast.InfixExpression", ie.Index)
	}
}

// --- order/combo/special ------------------------------------------------------

func TestIfStatementBareOrder(t *testing.T) {
	program := parseProgram(t, "order (x < y) { serve x }\n")
	stmt := singleStatement(t, program)
	is, ok := stmt.(*ast.IfStatement)
	if !ok {
		t.Fatalf("statement is %T, want *ast.IfStatement", stmt)
	}
	if len(is.Combos) != 0 {
		t.Errorf("got %d combos, want 0", len(is.Combos))
	}
	if is.Alternative != nil {
		t.Errorf("Alternative = %v, want nil", is.Alternative)
	}
}

func TestIfStatementFullChain(t *testing.T) {
	input := `order (a) { serve 1 } combo (b) { serve 2 } combo (c) { serve 3 } special { serve 4 }` + "\n"
	program := parseProgram(t, input)
	stmt := singleStatement(t, program)
	is := stmt.(*ast.IfStatement)
	if len(is.Combos) != 2 {
		t.Fatalf("got %d combos, want 2", len(is.Combos))
	}
	if is.Alternative == nil {
		t.Fatal("Alternative = nil, want a block")
	}
}

// --- knead loops ---------------------------------------------------------------

func TestForEachLoop(t *testing.T) {
	program := parseProgram(t, "knead item in xs { serve item }\n")
	stmt := singleStatement(t, program)
	fel, ok := stmt.(*ast.ForEachLoop)
	if !ok {
		t.Fatalf("statement is %T, want *ast.ForEachLoop", stmt)
	}
	if fel.Identifier.Value != "item" {
		t.Errorf("Identifier = %q, want \"item\"", fel.Identifier.Value)
	}
	testIdentifier(t, fel.Collection, "xs")
}

func TestCountedLoopFull(t *testing.T) {
	program := parseProgram(t, "knead (i = 0; i < 10; i++) { serve i }\n")
	stmt := singleStatement(t, program)
	cl, ok := stmt.(*ast.CountedLoop)
	if !ok {
		t.Fatalf("statement is %T, want *ast.CountedLoop", stmt)
	}
	if cl.Init == nil {
		t.Error("Init = nil, want an AssignStatement")
	}
	if cl.Cond == nil {
		t.Error("Cond = nil, want an expression")
	}
	if cl.Post == nil {
		t.Error("Post = nil, want an IncDecStatement")
	}
}

func TestCountedLoopAllClausesEmpty(t *testing.T) {
	program := parseProgram(t, "knead (;;) { burnt }\n")
	stmt := singleStatement(t, program)
	cl, ok := stmt.(*ast.CountedLoop)
	if !ok {
		t.Fatalf("statement is %T, want *ast.CountedLoop", stmt)
	}
	if cl.Init != nil || cl.Cond != nil || cl.Post != nil {
		t.Errorf("got Init=%v Cond=%v Post=%v, want all nil", cl.Init, cl.Cond, cl.Post)
	}
}

func TestCountedLoopOnlyCond(t *testing.T) {
	program := parseProgram(t, "knead (; i < 10; ) { flip }\n")
	stmt := singleStatement(t, program)
	cl := stmt.(*ast.CountedLoop)
	if cl.Init != nil {
		t.Errorf("Init = %v, want nil", cl.Init)
	}
	if cl.Cond == nil {
		t.Error("Cond = nil, want an expression")
	}
	if cl.Post != nil {
		t.Errorf("Post = %v, want nil", cl.Post)
	}
}

// --- bake ----------------------------------------------------------------------

func TestBakeStatement(t *testing.T) {
	program := parseProgram(t, "bake (x < 10) { x++ }\n")
	stmt := singleStatement(t, program)
	bs, ok := stmt.(*ast.BakeStatement)
	if !ok {
		t.Fatalf("statement is %T, want *ast.BakeStatement", stmt)
	}
	if bs.Condition == nil {
		t.Error("Condition = nil")
	}
	if len(bs.Body.Statements) != 1 {
		t.Fatalf("Body has %d statements, want 1", len(bs.Body.Statements))
	}
}

// --- Blocks as statements -------------------------------------------------------

func TestStandaloneBlockStatement(t *testing.T) {
	program := parseProgram(t, "{\nx = 1\n}\n")
	stmt := singleStatement(t, program)
	if _, ok := stmt.(*ast.BlockStatement); !ok {
		t.Fatalf("statement is %T, want *ast.BlockStatement", stmt)
	}
}

// --- Multi-statement programs ---------------------------------------------------

func TestProgramWithMultipleStatements(t *testing.T) {
	input := `x = 1
y = 2
serve x + y
`
	program := parseProgram(t, input)
	if len(program.Statements) != 3 {
		t.Fatalf("got %d statements, want 3", len(program.Statements))
	}
}

// --- Comprehensive smoke test ----------------------------------------------------

func TestParsesTheWorksExample(t *testing.T) {
	// Mirrors the lexer's TestExampleFiles: every .crust file under
	// examples/ must parse with zero errors, not just lex cleanly.
	program := parseProgram(t, theWorksSource)
	if len(program.Statements) == 0 {
		t.Fatal("expected at least one top-level statement")
	}
}

const theWorksSource = `
recipe sum(nums) {
	total = 0
	knead i in nums {
		total = total + i
	}
	serve total
}

order (sum([1, 2, 3]) == 6) {
	serve stuffed
} combo (thin) {
	serve thin
} special {
	serve nobox
}

knead (i = 0; i < 10; i++) {
	order (i % 2 == 0) {
		flip
	}
	order (i > 7) {
		burnt
	}
}

x = 5 (| "big" |) "small"
y = x ?: 0
z = 1..5
w = toppings{1, 2, 3}
m = {"a": 1, "b": 2}
`
