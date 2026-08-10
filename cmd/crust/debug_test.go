package main

import (
	"bytes"
	"io"
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

func TestParseDebugArgsYear(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantYear int
		wantErr  bool
	}{
		{"no year", []string{"day01.crust"}, 0, false},
		{"equals form", []string{"day01.crust", "--year=2020"}, 2020, false},
		{"space form", []string{"day01.crust", "--year", "2020"}, 2020, false},
		{"bad value", []string{"day01.crust", "--year=nope"}, 0, true},
		{"missing value", []string{"day01.crust", "--year"}, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, opts, err := parseDebugArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseDebugArgs(%v) = %+v, want an error", tt.args, opts)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDebugArgs(%v) unexpected error: %v", tt.args, err)
			}
			if opts.Year != tt.wantYear {
				t.Errorf("parseDebugArgs(%v).Year = %d, want %d", tt.args, opts.Year, tt.wantYear)
			}
		})
	}
}

// TestRunDebugPersistsExplicitYear confirms an explicit `crust develop
// --year` sticks for next time — the one place Year's persistence
// genuinely differs from Store's own "only saved once something's
// actually run from the Run tab" behavior, since there's no
// interactive equivalent for setting a year.
func TestRunDebugPersistsExplicitYear(t *testing.T) {
	withTempDevelStateDir(t)
	path := writeDebugFile(t, `deliver("hi")`)

	var stdout, stderr bytes.Buffer
	code := runDebug(path, debugOptions{Year: 2020, Plain: true}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}

	saved := loadDevelState()[mustAbs(t, path)]
	if saved.Year != 2020 {
		t.Errorf("saved Year = %d, want 2020", saved.Year)
	}
}

// TestRunDebugUsesSavedYearWhenNoneGiven mirrors
// TestRunDebugUsesSavedStoreWhenNoneGiven for Year: once persisted,
// reopening the file with no --year at all should still resolve to the
// remembered one.
func TestRunDebugUsesSavedYearWhenNoneGiven(t *testing.T) {
	withTempDevelStateDir(t)
	path := writeDebugFile(t, `deliver("hi")`)
	if err := saveDevelState(mustAbs(t, path), develState{Year: 2020}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runDebug(path, debugOptions{Plain: true}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}

	// Still 2020 afterward -- resolving from the saved state shouldn't
	// have overwritten it with defaultAoCYear.
	saved := loadDevelState()[mustAbs(t, path)]
	if saved.Year != 2020 {
		t.Errorf("saved Year = %d, want 2020 (should be untouched)", saved.Year)
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

// TestRunDebugUsesSavedStoreWhenNoneGiven confirms applySavedOptions is
// actually wired into runDebug: with no explicit --store, a
// remembered store for this file (debug_state.go) should be used for
// the very first recording, same as passing --store=part2 by hand
// would.
func TestRunDebugUsesSavedStoreWhenNoneGiven(t *testing.T) {
	withTempDevelStateDir(t)
	path := writeDebugFile(t, `
recipe store_part1() { deliver("one") }
recipe store_part2() { deliver("two") }
`)
	if err := saveDevelState(mustAbs(t, path), develState{Store: "part2"}); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runDebug(path, debugOptions{}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	out := stdout.String()
	if strings.Contains(out, `deliver("one")`) {
		t.Errorf("stdout = %q, should not have traced store_part1's body", out)
	}
	if !strings.Contains(out, `deliver("two")`) {
		t.Errorf("stdout = %q, want the remembered store_part2 traced", out)
	}
}

// TestRunDebugNoDefaultEntryPointListsAvailableOnes is a regression
// test for a real bug: a file with store_part1/store_part2 but no
// bare store(), run with no --store, used to silently record nothing
// but the top-level recipe declarations -- indistinguishable from
// `develop` itself being broken. It should behave like `crust run`'s
// equivalent case: fail with a message listing the entry points that
// do exist, not silently produce an empty-looking recording.
func TestRunDebugNoDefaultEntryPointListsAvailableOnes(t *testing.T) {
	path := writeDebugFile(t, `
recipe store_part1() { deliver("one") }
recipe store_part2() { deliver("two") }
`)
	var stdout, stderr bytes.Buffer
	code := runDebug(path, debugOptions{}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout = %q", code, stdout.String())
	}
	if !strings.Contains(stderr.String(), "no default entry point; pick one: --store=part1, --store=part2") {
		t.Errorf("stderr = %q, want it to list the available entry points", stderr.String())
	}
}

// TestRunDebugUnknownStoreNameIsAnError is the explicit-but-wrong
// counterpart: --store=<name> naming a recipe that doesn't exist
// should fail clearly rather than silently recording nothing.
func TestRunDebugUnknownStoreNameIsAnError(t *testing.T) {
	path := writeDebugFile(t, `recipe store_part1() { deliver("one") }`)
	var stdout, stderr bytes.Buffer
	code := runDebug(path, debugOptions{Store: "bogus"}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout = %q", code, stdout.String())
	}
	if !strings.Contains(stderr.String(), `no entry point named "bogus"`) {
		t.Errorf("stderr = %q, want it to name the missing entry point", stderr.String())
	}
}

// TestEmptyDebugViewHasNoRecordedSteps is emptyDebugView's whole
// purpose: the interactive TUI's starting point (unlike buildDebugView)
// runs nothing at all, so its Recorder should show zero steps and an
// empty tree -- confirmed against the file's own real entry points,
// which still work (entryPoints() does its own independent parse).
func TestEmptyDebugViewHasNoRecordedSteps(t *testing.T) {
	path := writeDebugFile(t, `recipe store_part1() { deliver("one") }
recipe store_part2() { deliver("two") }`)

	view, err := emptyDebugView(path)
	if err != nil {
		t.Fatalf("emptyDebugView: %v", err)
	}
	if view.rec.Steps() != 0 {
		t.Errorf("Steps() = %d, want 0 -- nothing should have run", view.rec.Steps())
	}
	if len(view.rec.Roots()) != 0 {
		t.Errorf("Roots() = %v, want empty -- nothing should have run", view.rec.Roots())
	}
	if got := view.entryPoints(); len(got) != 2 {
		t.Errorf("entryPoints() = %v, want 2 (entryPoints does its own parse, independent of the recording)", got)
	}
}

func TestEmptyDebugViewMissingFileReturnsError(t *testing.T) {
	if _, err := emptyDebugView("/no/such/file.crust"); err == nil {
		t.Error("emptyDebugView() = nil error, want an error for a missing file")
	}
}

func TestEmptyDebugViewParseErrorReturnsError(t *testing.T) {
	path := writeDebugFile(t, "x = = =\n")
	_, err := emptyDebugView(path)
	if err == nil {
		t.Fatal("emptyDebugView() = nil error, want a parse error")
	}
	if !strings.Contains(err.Error(), "parse error") {
		t.Errorf("err = %q, want it to mention a parse error", err.Error())
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

// TestRunDebugMissingFile covers a missing *parent directory*
// specifically -- ensureFileExists deliberately never creates one (see
// its own doc comment), so this stays a real error even after
// auto-create landed; TestRunDebugAutoCreatesMissingFile below covers
// the actual auto-create case (parent directory exists, only the file
// itself doesn't).
func TestRunDebugMissingFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runDebug("/no/such/file.crust", debugOptions{}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestEnsureFileExistsCreatesAMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "newday.crust")
	var stderr bytes.Buffer
	if err := ensureFileExists(path, &stderr); err != nil {
		t.Fatalf("ensureFileExists: %s", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file was not created: %s", err)
	}
	if len(data) != 0 {
		t.Errorf("created file has %d bytes, want empty", len(data))
	}
	if !strings.Contains(stderr.String(), "doesn't exist yet") {
		t.Errorf("stderr = %q, want a note that the file was created", stderr.String())
	}
}

func TestEnsureFileExistsLeavesAnExistingFileAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "day01.crust")
	if err := os.WriteFile(path, []byte(`deliver("already here")`), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if err := ensureFileExists(path, &stderr); err != nil {
		t.Fatalf("ensureFileExists: %s", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `deliver("already here")` {
		t.Errorf("existing file was modified: %q", data)
	}
	if stderr.String() != "" {
		t.Errorf("stderr = %q, want nothing printed for a file that already existed", stderr.String())
	}
}

func TestEnsureFileExistsMissingParentDirIsAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-subdir", "day01.crust")
	var stderr bytes.Buffer
	if err := ensureFileExists(path, &stderr); err == nil {
		t.Error("expected an error for a missing parent directory, got nil")
	}
}

// TestRunDebugAutoCreatesMissingFile is the actual feature: `crust
// develop newday.crust` on a file that hasn't been written yet, in a
// directory that does exist, creates it and proceeds -- exit 0, not
// the missing-file error TestRunDebugMissingFile above still (and
// correctly) gets for a missing parent directory.
func TestRunDebugAutoCreatesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "newday.crust")
	var stdout, stderr bytes.Buffer
	code := runDebug(path, debugOptions{Plain: true}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file was not created: %s", err)
	}
	if !strings.Contains(stdout.String(), "0 steps") {
		t.Errorf("stdout = %q, want the (empty, harmless) recording of a freshly-created empty file", stdout.String())
	}
}

// TestRunDebugAutoCreateWorksInTUIPath confirms auto-create is applied
// before runDebug branches into the TUI path too, not just --plain --
// emptyDebugView itself has no auto-create logic of its own (that's
// deliberately runDebug's job, applied once ahead of both branches),
// so it still fails on a path that doesn't exist yet; only after
// ensureFileExists runs does the same call succeed.
func TestRunDebugAutoCreateWorksInTUIPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "newday.crust")
	if _, err := emptyDebugView(path); err == nil {
		t.Fatal("expected emptyDebugView to fail on a file that doesn't exist yet (auto-create lives in runDebug, not here)")
	}
	if err := ensureFileExists(path, io.Discard); err != nil {
		t.Fatalf("ensureFileExists: %s", err)
	}
	if _, err := emptyDebugView(path); err != nil {
		t.Fatalf("emptyDebugView after auto-create: %s", err)
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
