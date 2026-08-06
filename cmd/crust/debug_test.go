package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sintfoap/cRust/internal/debugger"
	"github.com/Sintfoap/cRust/internal/object"
)

func TestParseDebugArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantPath  string
		wantStore string
		wantMax   int
		wantPlain bool
		wantErr   bool
	}{
		{"file only", []string{"day01.crust"}, "day01.crust", "", 0, false, false},
		{"file then store", []string{"day01.crust", "--store=part1"}, "day01.crust", "part1", 0, false, false},
		{"plain flag", []string{"day01.crust", "--plain"}, "day01.crust", "", 0, true, false},
		{"max-steps equals form", []string{"day01.crust", "--max-steps=5"}, "day01.crust", "", 5, false, false},
		{"max-steps separate form", []string{"day01.crust", "--max-steps", "5"}, "day01.crust", "", 5, false, false},
		{"no args", nil, "", "", 0, false, true},
		{"bare --store with no value", []string{"day01.crust", "--store"}, "", "", 0, false, true},
		{"bad max-steps", []string{"day01.crust", "--max-steps=0"}, "", "", 0, false, true},
		{"max-steps missing value", []string{"day01.crust", "--max-steps"}, "", "", 0, false, true},
		{"unknown flag", []string{"day01.crust", "--bogus"}, "", "", 0, false, true},
		{"two positional args", []string{"day01.crust", "day02.crust"}, "", "", 0, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, opts, err := parseDebugArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseDebugArgs(%v) = %q, %+v, want an error", tt.args, path, opts)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDebugArgs(%v) unexpected error: %v", tt.args, err)
			}
			if path != tt.wantPath || opts.Store != tt.wantStore || opts.MaxSteps != tt.wantMax || opts.Plain != tt.wantPlain {
				t.Errorf("parseDebugArgs(%v) = %q, %+v, want path=%q store=%q max=%d plain=%v",
					tt.args, path, opts, tt.wantPath, tt.wantStore, tt.wantMax, tt.wantPlain)
			}
		})
	}
}

func writeDebugFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "prog.crust")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunDebugPlainProgramWithNoEntryPoint(t *testing.T) {
	path := writeDebugFile(t, "x = 1\ny = 2\n")
	var stdout, stderr bytes.Buffer
	// stdout here is a *bytes.Buffer, never a real terminal, so this
	// always takes the plain-text path regardless of opts.Plain --
	// exactly what makes this command testable at all.
	code := runDebug(path, debugOptions{}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "x = 1") || !strings.Contains(out, "y = 2") {
		t.Errorf("stdout = %q, want it to contain both statements", out)
	}
	if !strings.Contains(out, "2 steps") {
		t.Errorf("stdout = %q, want the header to report 2 steps", out)
	}
}

func TestRunDebugPlainMarksFrameCloseWithComment(t *testing.T) {
	path := writeDebugFile(t, `recipe store() {
    total = 1 + 1
    deliver(total)
}
`)
	var stdout, stderr bytes.Buffer
	code := runDebug(path, debugOptions{}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "// end store(...)") {
		t.Errorf("stdout = %q, want a // end closing marker for the store() entry-point frame", out)
	}
}

func TestRunDebugPlainOmitsCloseCommentForChildlessNode(t *testing.T) {
	path := writeDebugFile(t, "x = 1\n")
	var stdout, stderr bytes.Buffer
	code := runDebug(path, debugOptions{}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "// end") {
		t.Errorf("stdout = %q, a leaf statement should get no closing marker", stdout.String())
	}
}

func TestRunDebugCallsBareStoreEntryPoint(t *testing.T) {
	path := writeDebugFile(t, `recipe store() {
    total = 1 + 1
    deliver(total)
}
`)
	var stdout, stderr bytes.Buffer
	code := runDebug(path, debugOptions{}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "total = (1 + 1)") {
		t.Errorf("stdout = %q, want the entry point's body traced", out)
	}
	if !strings.Contains(out, "2") {
		t.Errorf("stdout = %q, want deliver's printed 2 in there somewhere", out)
	}
}

func TestRunDebugSelectsNamedStore(t *testing.T) {
	path := writeDebugFile(t, `
recipe store_part1() { deliver("one") }
recipe store_part2() { deliver("two") }
`)
	var stdout, stderr bytes.Buffer
	code := runDebug(path, debugOptions{Store: "part2"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	out := stdout.String()
	if strings.Contains(out, `deliver("one")`) {
		t.Errorf("stdout = %q, should not have traced store_part1's body", out)
	}
	if !strings.Contains(out, `deliver("two")`) {
		t.Errorf("stdout = %q, want store_part2's body traced", out)
	}
}

func TestRunDebugShowsRuntimeErrorInPlace(t *testing.T) {
	path := writeDebugFile(t, `recipe store() {
    x = 1 / 0
}
`)
	var stdout, stderr bytes.Buffer
	// A runtime Error, unlike a bad file path or a parse error, isn't
	// this command's own failure -- the whole point of `debug` is to
	// look at a run including its failure, so this must NOT be a
	// non-zero exit the way `crust run` would be.
	code := runDebug(path, debugOptions{}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (the error is shown, not treated as this command's own failure); stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "division by zero") {
		t.Errorf("stdout = %q, want the division-by-zero error shown in the trace", stdout.String())
	}
}

func TestRunDebugMissingFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runDebug("/no/such/file.crust", debugOptions{}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunDebugParseError(t *testing.T) {
	path := writeDebugFile(t, "x = = =\n")
	var stdout, stderr bytes.Buffer
	code := runDebug(path, debugOptions{}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "parse error") {
		t.Errorf("stderr = %q, want it to mention a parse error", stderr.String())
	}
}

func TestRunDebugMaxStepsTruncates(t *testing.T) {
	path := writeDebugFile(t, `recipe store() {
    knead i in 0.<10 {
        x = i
    }
}
`)
	var stdout, stderr bytes.Buffer
	code := runDebug(path, debugOptions{MaxSteps: 2}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "capped") {
		t.Errorf("stdout = %q, want the header to mention the recording was capped", stdout.String())
	}
}

func TestIsColorTerminalFalseForNonFile(t *testing.T) {
	if isColorTerminal(&bytes.Buffer{}) {
		t.Error("isColorTerminal(*bytes.Buffer) = true, want false")
	}
}

func TestSelfPctTextZeroOverall(t *testing.T) {
	if got := selfPctText(debugger.NodeTiming{}, 0); got != "—" {
		t.Errorf("selfPctText with zero overall = %q, want an em dash", got)
	}
}

func TestSizeTextUnknown(t *testing.T) {
	if got := sizeText(&debugger.Step{SizeOK: false}); got != "—" {
		t.Errorf("sizeText with SizeOK=false = %q, want an em dash", got)
	}
}

func TestSizeTextKnown(t *testing.T) {
	if got := sizeText(&debugger.Step{SizeOK: true, Size: 5}); got != "5" {
		t.Errorf("sizeText with Size=5 = %q, want %q", got, "5")
	}
}

func TestShortInspectNil(t *testing.T) {
	if got := shortInspect(nil); got != "nobox" {
		t.Errorf("shortInspect(nil) = %q, want %q", got, "nobox")
	}
}

func TestShortInspectTruncatesLongValues(t *testing.T) {
	s := &object.String{Value: strings.Repeat("x", 100)}
	got := shortInspect(s)
	if len([]rune(got)) != maxInspectRunes {
		t.Errorf("shortInspect truncated length = %d, want %d", len([]rune(got)), maxInspectRunes)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("shortInspect(long value) = %q, want it to end with an ellipsis", got)
	}
}
