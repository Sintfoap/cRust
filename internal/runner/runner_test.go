package runner

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/parser"
)

func TestRunPlainScriptNoEntryPoint(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(`deliver("hi")`+"\n", "", "prog.crust", "crust run: ", strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "hi") {
		t.Errorf("stdout = %q, want it to contain %q", stdout.String(), "hi")
	}
}

func TestRunBareStore(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(`recipe store() { deliver("default") }`+"\n", "", "prog.crust", "crust run: ", strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "default") {
		t.Errorf("stdout = %q, want it to contain %q", stdout.String(), "default")
	}
}

func TestRunStoreFlagSelectsNamedEntryPoint(t *testing.T) {
	src := "recipe store_part1() { deliver(\"one\") }\nrecipe store_part2() { deliver(\"two\") }\n"
	var stdout, stderr bytes.Buffer
	code := Run(src, "part2", "prog.crust", "crust run: ", strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr.String())
	}
	out := stdout.String()
	if strings.Contains(out, "one") || !strings.Contains(out, "two") {
		t.Errorf("stdout = %q, want only %q", out, "two")
	}
}

func TestRunUnknownStoreFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(`recipe store_part1() { deliver("one") }`+"\n", "nope", "prog.crust", "crust run: ", strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `no entry point named "nope"`) {
		t.Errorf("stderr = %q, want a no-entry-point-named message", stderr.String())
	}
	if !strings.HasPrefix(stderr.String(), "crust run: ") {
		t.Errorf("stderr = %q, want it prefixed with msgPrefix", stderr.String())
	}
}

func TestRunNoDefaultListsOptions(t *testing.T) {
	src := "recipe store_part1() { deliver(\"one\") }\nrecipe store_part2() { deliver(\"two\") }\n"
	var stdout, stderr bytes.Buffer
	code := Run(src, "", "prog.crust", "crust run: ", strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "--store=part1") || !strings.Contains(stderr.String(), "--store=part2") {
		t.Errorf("stderr = %q, want it to list both available entry points", stderr.String())
	}
}

func TestRunParseError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run("order (a < b {\n serve 1\n}\n", "", "prog.crust", "crust run: ", strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "parse error") {
		t.Errorf("stderr = %q, want a parse error message", stderr.String())
	}
}

func TestRunRuntimeErrorAtTopLevelIncludesPath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run("x = 1 / 0\n", "", "prog.crust", "crust run: ", strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "division by zero") {
		t.Errorf("stderr = %q, want a division-by-zero message", stderr.String())
	}
	if !strings.Contains(stderr.String(), "prog.crust:1:") {
		t.Errorf("stderr = %q, want it to include the path and line number", stderr.String())
	}
}

func TestRunRuntimeErrorWithEmptyPathOmitsPath(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run("x = 1 / 0\n", "", "", "", strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if strings.Contains(stderr.String(), ".crust") {
		t.Errorf("stderr = %q, want no path when path is empty", stderr.String())
	}
	if !strings.HasPrefix(stderr.String(), "1:") {
		t.Errorf("stderr = %q, want it to start with the bare line:col", stderr.String())
	}
}

func TestRunRuntimeErrorInsideEntryPoint(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run("recipe store() {\n deliver(1 / 0)\n}\n", "", "prog.crust", "crust run: ", strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "division by zero") {
		t.Errorf("stderr = %q, want a division-by-zero message", stderr.String())
	}
}

// TestRunRuntimeErrorPrintsCallChain confirms Run's stderr includes
// the indented "in ..., called from ..." lines FrameLines builds once
// an error unwound through more than one recipe call — the primary
// "path:line:col: message" line stays exactly as before (still the
// innermost failure's own position), with the chain underneath it.
func TestRunRuntimeErrorPrintsCallChain(t *testing.T) {
	src := "recipe divide(a, b) {\n serve a / b\n}\nrecipe store() {\n deliver(divide(1, 0))\n}\n"
	var stdout, stderr bytes.Buffer
	code := Run(src, "", "prog.crust", "crust run: ", strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	got := stderr.String()
	if !strings.Contains(got, "division by zero") {
		t.Fatalf("stderr = %q, want a division-by-zero message", got)
	}
	if !strings.Contains(got, "in divide(...), called from 5:") {
		t.Errorf("stderr = %q, want a call-chain line for divide(...)", got)
	}
	if !strings.Contains(got, "in store(...)") {
		t.Errorf("stderr = %q, want a call-chain line for store(...)", got)
	}
}

// TestRunRuntimeErrorSingleLevelNoCallChain confirms a single level of
// call wrapping (a failure directly inside store()'s own body, no
// further nesting) prints exactly as it did before this feature
// existed — no "in store()" line, since it adds nothing beyond the
// primary line's own position.
func TestRunRuntimeErrorSingleLevelNoCallChain(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run("recipe store() {\n deliver(1 / 0)\n}\n", "", "prog.crust", "crust run: ", strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if strings.Contains(stderr.String(), "in store") {
		t.Errorf("stderr = %q, want no call-chain line for a single level of wrapping", stderr.String())
	}
}

func TestRunMsgPrefixEmpty(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run("order (a < b {\n serve 1\n}\n", "", "prog.crust", "", strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if got := stderr.String(); got == "" || got[0] < '0' || got[0] > '9' {
		t.Errorf("stderr = %q, want it to start with the parse-error count digit (no msgPrefix)", got)
	}
}

func TestCollectEntryPoints(t *testing.T) {
	src := "recipe store() {}\nrecipe store_part1() {}\nrecipe helper() {}\n"
	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("unexpected parse errors: %v", p.Errors())
	}
	got := CollectEntryPoints(program)
	want := []string{"", "part1"}
	if len(got) != len(want) {
		t.Fatalf("CollectEntryPoints() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("CollectEntryPoints()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestStoreFlags(t *testing.T) {
	got := StoreFlags([]string{"", "part1", "part2"})
	want := []string{"(default)", "--store=part1", "--store=part2"}
	if len(got) != len(want) {
		t.Fatalf("StoreFlags() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("StoreFlags()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
