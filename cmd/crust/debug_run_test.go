package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRunInputInsertAndString(t *testing.T) {
	var in runInputModel
	for _, r := range "day01.txt" {
		in.insert(r)
	}
	if got := in.String(); got != "day01.txt" {
		t.Errorf("String() = %q, want %q", got, "day01.txt")
	}
	if in.cursor != len("day01.txt") {
		t.Errorf("cursor = %d, want %d (end of input)", in.cursor, len("day01.txt"))
	}
}

func TestRunInputInsertAtCursorMidString(t *testing.T) {
	in := runInputModel{value: []rune("day1.txt"), cursor: 3}
	in.insert('0')
	if got := in.String(); got != "day01.txt" {
		t.Errorf("String() = %q, want %q", got, "day01.txt")
	}
	if in.cursor != 4 {
		t.Errorf("cursor = %d, want 4", in.cursor)
	}
}

func TestRunInputBackspace(t *testing.T) {
	in := runInputModel{value: []rune("abc"), cursor: 3}
	in.backspace()
	if got := in.String(); got != "ab" {
		t.Errorf("String() = %q, want %q", got, "ab")
	}
	if in.cursor != 2 {
		t.Errorf("cursor = %d, want 2", in.cursor)
	}
}

func TestRunInputBackspaceAtStartIsNoOp(t *testing.T) {
	in := runInputModel{value: []rune("abc"), cursor: 0}
	in.backspace()
	if got := in.String(); got != "abc" {
		t.Errorf("String() = %q, want unchanged %q", got, "abc")
	}
}

func TestRunInputDeleteForward(t *testing.T) {
	in := runInputModel{value: []rune("abc"), cursor: 1}
	in.deleteForward()
	if got := in.String(); got != "ac" {
		t.Errorf("String() = %q, want %q", got, "ac")
	}
}

func TestRunInputDeleteForwardAtEndIsNoOp(t *testing.T) {
	in := runInputModel{value: []rune("abc"), cursor: 3}
	in.deleteForward()
	if got := in.String(); got != "abc" {
		t.Errorf("String() = %q, want unchanged %q", got, "abc")
	}
}

func TestRunInputLeftRightClampAtEdges(t *testing.T) {
	in := runInputModel{value: []rune("ab"), cursor: 0}
	in.left()
	if in.cursor != 0 {
		t.Errorf("cursor after left-at-start = %d, want 0", in.cursor)
	}
	in.right()
	in.right()
	in.right() // past the end
	if in.cursor != 2 {
		t.Errorf("cursor after overshooting right = %d, want 2 (len)", in.cursor)
	}
}

func TestRunInputRenderShowsCursorAtEnd(t *testing.T) {
	in := runInputModel{value: []rune("ab"), cursor: 2}
	out := in.render()
	if !strings.HasPrefix(out, "ab") {
		t.Errorf("render() = %q, want it to start with the typed text", out)
	}
}

func TestRunInputRenderShowsCursorMidString(t *testing.T) {
	in := runInputModel{value: []rune("ab"), cursor: 0}
	out := in.render()
	if !strings.Contains(out, "a") || !strings.Contains(out, "b") {
		t.Errorf("render() = %q, want both characters present", out)
	}
}

func TestHandleRunTabKeyTypingInsertsReservedLetters(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabRun
	for _, r := range []rune("q") {
		next, _ := m.handleRunTabKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(debugModel)
	}
	if got := m.runInput.String(); got != "q" {
		t.Errorf("runInput = %q, want %q (q should type, not quit, on the Run tab)", got, "q")
	}
	if m.active != tabRun {
		t.Error("typing q on the Run tab should not have quit or changed tabs")
	}
}

func TestHandleRunTabKeySpaceInserts(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabRun
	m.runInput = runInputModel{value: []rune("a"), cursor: 1}
	next, _ := m.handleRunTabKey(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
	m = next.(debugModel)
	if got := m.runInput.String(); got != "a " {
		t.Errorf("runInput = %q, want %q", got, "a ")
	}
}

func TestHandleRunTabKeyBackspaceEditsField(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabRun
	m.runInput = runInputModel{value: []rune("abc"), cursor: 3}
	next, _ := m.handleRunTabKey(tea.KeyMsg{Type: tea.KeyBackspace})
	m = next.(debugModel)
	if got := m.runInput.String(); got != "ab" {
		t.Errorf("runInput = %q, want %q", got, "ab")
	}
}

func TestHandleRunTabKeyHomeEndMoveCursor(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabRun
	m.runInput = runInputModel{value: []rune("abc"), cursor: 1}
	next, _ := m.handleRunTabKey(tea.KeyMsg{Type: tea.KeyHome})
	m = next.(debugModel)
	if m.runInput.cursor != 0 {
		t.Errorf("cursor after Home = %d, want 0", m.runInput.cursor)
	}
	next, _ = m.handleRunTabKey(tea.KeyMsg{Type: tea.KeyEnd})
	m = next.(debugModel)
	if m.runInput.cursor != 3 {
		t.Errorf("cursor after End = %d, want 3", m.runInput.cursor)
	}
}

func TestHandleRunTabKeyLeftRightMoveCursor(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabRun
	m.runInput = runInputModel{value: []rune("abc"), cursor: 1}
	next, _ := m.handleRunTabKey(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(debugModel)
	if m.runInput.cursor != 2 {
		t.Errorf("cursor after Right = %d, want 2", m.runInput.cursor)
	}
	next, _ = m.handleRunTabKey(tea.KeyMsg{Type: tea.KeyLeft})
	m = next.(debugModel)
	if m.runInput.cursor != 1 {
		t.Errorf("cursor after Left = %d, want 1", m.runInput.cursor)
	}
}

func TestHandleRunTabKeyDeleteEditsField(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabRun
	m.runInput = runInputModel{value: []rune("abc"), cursor: 0}
	next, _ := m.handleRunTabKey(tea.KeyMsg{Type: tea.KeyDelete})
	m = next.(debugModel)
	if got := m.runInput.String(); got != "bc" {
		t.Errorf("runInput = %q, want %q", got, "bc")
	}
}

func TestHandleRunTabKeyTabSwitchesTabs(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabRun
	next, _ := m.handleRunTabKey(tea.KeyMsg{Type: tea.KeyTab})
	m = next.(debugModel)
	if m.active != tabNav {
		t.Errorf("active = %v, want tabNav (Run's own next tab)", m.active)
	}
}

func TestHandleRunTabKeyShiftTabSwitchesTabs(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabRun
	next, _ := m.handleRunTabKey(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = next.(debugModel)
	if m.active != tabEditor {
		t.Errorf("active = %v, want tabEditor", m.active)
	}
}

func TestHandleRunTabKeyEnterTriggersRunCmd(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabRun
	_, cmd := m.handleRunTabKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("enter on the Run tab should return a non-nil Cmd")
	}
}

func TestHandleRunTabKeyCtrlCQuits(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabRun
	_, cmd := m.handleRunTabKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd for ctrl+c")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("ctrl+c on the Run tab should quit")
	}
}

func TestHandleRunTabKeyEscQuits(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabRun
	_, cmd := m.handleRunTabKey(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd for esc")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("esc on the Run tab should quit")
	}
}

func TestRunProgramCmdNoInputFile(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.view.path = writeDebugFile(t, `deliver("hello")`)
	msg := m.runProgramCmd()().(runResultMsg)
	if msg.err != nil {
		t.Fatalf("unexpected err: %v", msg.err)
	}
	if !strings.Contains(msg.stdout, "hello") {
		t.Errorf("stdout = %q, want it to contain hello", msg.stdout)
	}
	if msg.code != 0 {
		t.Errorf("code = %d, want 0", msg.code)
	}
}

func TestRunProgramCmdWithInputFile(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(inputPath, []byte("42\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newDebugModel(viewFor(t, "x = 1"))
	m.view.path = writeDebugFile(t, `deliver(unbox())`)
	m.runInput = runInputModel{value: []rune(inputPath)}
	msg := m.runProgramCmd()().(runResultMsg)
	if msg.err != nil {
		t.Fatalf("unexpected err: %v", msg.err)
	}
	if !strings.Contains(msg.stdout, "42") {
		t.Errorf("stdout = %q, want it to contain the input file's contents", msg.stdout)
	}
}

func TestRunProgramCmdMissingInputFileReturnsErr(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.view.path = writeDebugFile(t, "x = 1\n")
	m.runInput = runInputModel{value: []rune("/does/not/exist.txt")}
	msg := m.runProgramCmd()().(runResultMsg)
	if msg.err == nil {
		t.Fatal("expected an error for a missing input file")
	}
}

func TestRunProgramCmdRuntimeErrorReflectedInCode(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.view.path = writeDebugFile(t, "x = 1 / 0")
	msg := m.runProgramCmd()().(runResultMsg)
	if msg.err != nil {
		t.Fatalf("unexpected file-open err: %v", msg.err)
	}
	if msg.code == 0 {
		t.Error("expected a non-zero exit code for a runtime error")
	}
	if msg.stderr == "" {
		t.Error("expected stderr to report the runtime error")
	}
}

func TestHandleRunResultFileErrorSetsFailedOutput(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	next, cmd := m.handleRunResult(runResultMsg{err: os.ErrNotExist})
	m = next.(debugModel)
	if !m.runFailed {
		t.Error("expected runFailed after a file-open error")
	}
	if m.runOutput == "" {
		t.Error("expected runOutput to report the error")
	}
	if cmd != nil {
		t.Error("handleRunResult should not return a follow-up Cmd")
	}
}

func TestHandleRunResultSuccessCombinesStdoutStderr(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	next, _ := m.handleRunResult(runResultMsg{stdout: "out\n", stderr: "warn\n", code: 0})
	m = next.(debugModel)
	if !strings.Contains(m.runOutput, "out") || !strings.Contains(m.runOutput, "warn") {
		t.Errorf("runOutput = %q, want both stdout and stderr present", m.runOutput)
	}
	if m.runFailed {
		t.Error("code 0 should not mark the run as failed")
	}
}

func TestHandleRunResultNoOutputShowsPlaceholder(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	next, _ := m.handleRunResult(runResultMsg{code: 0})
	m = next.(debugModel)
	if !strings.Contains(m.runOutput, "no output") {
		t.Errorf("runOutput = %q, want a no-output placeholder", m.runOutput)
	}
}

func TestHandleRunResultNonZeroCodeMarksFailed(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	next, _ := m.handleRunResult(runResultMsg{stderr: "boom", code: 1})
	m = next.(debugModel)
	if !m.runFailed {
		t.Error("expected runFailed for a non-zero exit code")
	}
}

// TestHandleRunResultAppliesRetracedView confirms a successful retrace
// replaces the model's view (what Time/Memory/Stepper read from),
// makes the run's store "current" in m.opts, and resets the Stepper's
// fold/cursor state the same way a successful Editor reload does.
func TestHandleRunResultAppliesRetracedView(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.cursor, m.top = 5, 2
	newView := viewFor(t, `recipe store_part2() { deliver("two") }`)

	next, cmd := m.handleRunResult(runResultMsg{stdout: "two\n", code: 0, view: newView, store: "part2"})
	m = next.(debugModel)

	if m.view != newView {
		t.Error("expected m.view to be replaced with the retraced view")
	}
	if m.opts.Store != "part2" {
		t.Errorf("m.opts.Store = %q, want %q", m.opts.Store, "part2")
	}
	if m.cursor != 0 || m.top != 0 {
		t.Errorf("cursor/top = %d/%d, want reset to 0/0", m.cursor, m.top)
	}
	if cmd != nil {
		t.Error("handleRunResult should not return a follow-up Cmd")
	}
}

// TestHandleRunResultNilViewLeavesExistingViewAlone confirms a failed
// retrace (view nil, e.g. the debugged file vanished mid-run) doesn't
// blank out or otherwise disturb whatever the Time/Memory/Stepper tabs
// were already showing.
func TestHandleRunResultNilViewLeavesExistingViewAlone(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	original := m.view

	next, _ := m.handleRunResult(runResultMsg{stdout: "hi\n", code: 0})
	m = next.(debugModel)

	if m.view != original {
		t.Error("expected m.view to stay unchanged when the retrace produced no view")
	}
}

func TestViewRunShowsInstructionsBeforeAnyRun(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.view.path = "day01.crust"
	out := m.viewRun()
	if !strings.Contains(out, "day01.crust") {
		t.Errorf("viewRun() = %q, want it to mention the file being debugged", out)
	}
}

func TestViewRunShowsOutputAfterARun(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.runOutput = "hello\n"
	out := m.viewRun()
	if !strings.Contains(out, "hello") {
		t.Errorf("viewRun() = %q, want the captured output", out)
	}
}

func TestDebugModelUpdateDispatchesRunResultMsg(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	next, _ := m.Update(runResultMsg{stdout: "hi"})
	if next.(debugModel).runOutput != "hi" {
		t.Error("Update() should route runResultMsg to handleRunResult")
	}
}

func TestScanEntryPointsFindsStoreAndNamed(t *testing.T) {
	path := writeDebugFile(t, `
recipe store_part1() {
    deliver(1)
}
recipe store_part2() {
    deliver(2)
}
`)
	got := scanEntryPoints(path)
	want := map[string]bool{"part1": true, "part2": true}
	if len(got) != 2 {
		t.Fatalf("scanEntryPoints() = %v, want 2 entries", got)
	}
	for _, g := range got {
		if !want[g] {
			t.Errorf("unexpected entry point %q", g)
		}
	}
}

func TestScanEntryPointsBareStore(t *testing.T) {
	path := writeDebugFile(t, `recipe store() { deliver(1) }`)
	got := scanEntryPoints(path)
	if len(got) != 1 || got[0] != "" {
		t.Errorf("scanEntryPoints() = %v, want [\"\"] (the bare store)", got)
	}
}

func TestScanEntryPointsNoneFound(t *testing.T) {
	path := writeDebugFile(t, "x = 1\n")
	if got := scanEntryPoints(path); got != nil {
		t.Errorf("scanEntryPoints() = %v, want nil for a file with no entry points", got)
	}
}

func TestScanEntryPointsParseErrorReturnsNil(t *testing.T) {
	path := writeDebugFile(t, "x = (\n")
	if got := scanEntryPoints(path); got != nil {
		t.Errorf("scanEntryPoints() = %v, want nil for an unparseable file", got)
	}
}

func TestScanEntryPointsMissingFileReturnsNil(t *testing.T) {
	if got := scanEntryPoints("/does/not/exist.crust"); got != nil {
		t.Errorf("scanEntryPoints() = %v, want nil for a missing file", got)
	}
}

func TestDebugViewEntryPointsIsCached(t *testing.T) {
	path := writeDebugFile(t, `recipe store_part1() { deliver(1) }`)
	v := &debugView{path: path}
	first := v.entryPoints()
	// Change the file on disk; entryPoints should still return the
	// cached answer rather than re-scanning.
	if err := os.WriteFile(path, []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second := v.entryPoints()
	if len(first) != 1 || len(second) != 1 {
		t.Errorf("entryPoints() changed after the file changed on disk: first=%v second=%v", first, second)
	}
}

func TestIndexOfEntryFound(t *testing.T) {
	options := []string{"", "part1", "part2"}
	if got := indexOfEntry(options, "part2"); got != 2 {
		t.Errorf("indexOfEntry(_, part2) = %d, want 2", got)
	}
}

func TestIndexOfEntryNotFoundDefaultsToZero(t *testing.T) {
	options := []string{"part1", "part2"}
	if got := indexOfEntry(options, "nope"); got != 0 {
		t.Errorf("indexOfEntry(_, nope) = %d, want 0", got)
	}
}

func TestIndexOfEntryEmptyOptions(t *testing.T) {
	if got := indexOfEntry(nil, "part1"); got != 0 {
		t.Errorf("indexOfEntry(nil, _) = %d, want 0", got)
	}
}

func TestRunEntryOptionsReflectsView(t *testing.T) {
	path := writeDebugFile(t, `recipe store_part1() { deliver(1) }
recipe store_part2() { deliver(2) }`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	got := m.runEntryOptions()
	if len(got) != 2 {
		t.Errorf("runEntryOptions() = %v, want 2 entries", got)
	}
}

func TestSelectedRunEntryNoOptionsIsEmpty(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	if got := m.selectedRunEntry(); got != "" {
		t.Errorf("selectedRunEntry() = %q, want \"\" when there are no entry points", got)
	}
}

func TestSelectedRunEntryReturnsSelectedOption(t *testing.T) {
	path := writeDebugFile(t, `recipe store_part1() { deliver(1) }
recipe store_part2() { deliver(2) }`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.runEntryIndex = 1
	got := m.selectedRunEntry()
	options := m.runEntryOptions()
	if got != options[1] {
		t.Errorf("selectedRunEntry() = %q, want %q", got, options[1])
	}
}

func TestSelectedRunEntryOutOfRangeFallsBackToFirst(t *testing.T) {
	path := writeDebugFile(t, `recipe store_part1() { deliver(1) }`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.runEntryIndex = 99
	if got := m.selectedRunEntry(); got != "part1" {
		t.Errorf("selectedRunEntry() = %q, want %q (fallback to index 0)", got, "part1")
	}
}

func TestToggleRunFocusWithOptions(t *testing.T) {
	path := writeDebugFile(t, `recipe store_part1() { deliver(1) }`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	if m.runEntryFocused {
		t.Fatal("expected runEntryFocused to start false")
	}
	m.toggleRunFocus()
	if !m.runEntryFocused {
		t.Error("expected runEntryFocused true after one toggle")
	}
	m.toggleRunFocus()
	if m.runEntryFocused {
		t.Error("expected runEntryFocused false after a second toggle")
	}
}

func TestToggleRunFocusNoOptionsIsNoOp(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.toggleRunFocus()
	if m.runEntryFocused {
		t.Error("toggleRunFocus should be a no-op when there are no entry points")
	}
}

func TestCycleRunEntryWraps(t *testing.T) {
	path := writeDebugFile(t, `recipe store_part1() { deliver(1) }
recipe store_part2() { deliver(2) }`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.runEntryIndex = 0
	m.cycleRunEntry(-1)
	if m.runEntryIndex != 1 {
		t.Errorf("runEntryIndex after wrapping backward = %d, want 1 (len-1)", m.runEntryIndex)
	}
	m.cycleRunEntry(1)
	if m.runEntryIndex != 0 {
		t.Errorf("runEntryIndex after wrapping forward = %d, want 0", m.runEntryIndex)
	}
}

func TestCycleRunEntryNoOptionsIsNoOp(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.cycleRunEntry(1)
	if m.runEntryIndex != 0 {
		t.Errorf("runEntryIndex = %d, want unchanged 0", m.runEntryIndex)
	}
}

func TestHandleRunTabKeyUpDownTogglesFocus(t *testing.T) {
	path := writeDebugFile(t, `recipe store_part1() { deliver(1) }`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.active = tabRun
	next, _ := m.handleRunTabKey(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(debugModel)
	if !m.runEntryFocused {
		t.Error("Down should move focus to the entry-point row")
	}
}

func TestHandleRunTabKeyLeftRightCyclesEntryWhenFocused(t *testing.T) {
	path := writeDebugFile(t, `recipe store_part1() { deliver(1) }
recipe store_part2() { deliver(2) }`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.active = tabRun
	m.runEntryFocused = true
	next, _ := m.handleRunTabKey(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(debugModel)
	if m.runEntryIndex != 1 {
		t.Errorf("runEntryIndex after Right = %d, want 1", m.runEntryIndex)
	}
}

func TestHandleRunTabKeyTypingIgnoredWhenEntryFocused(t *testing.T) {
	path := writeDebugFile(t, `recipe store_part1() { deliver(1) }`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.active = tabRun
	m.runEntryFocused = true
	next, _ := m.handleRunTabKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = next.(debugModel)
	if m.runInput.String() != "" {
		t.Errorf("runInput = %q, want unchanged when the entry-point row has focus", m.runInput.String())
	}
}

func TestRunProgramCmdUsesSelectedEntry(t *testing.T) {
	path := writeDebugFile(t, `recipe store_part1() {
    deliver("one")
}
recipe store_part2() {
    deliver("two")
}`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.runEntryIndex = 1 // part2
	msg := m.runProgramCmd()().(runResultMsg)
	if !strings.Contains(msg.stdout, "two") {
		t.Errorf("stdout = %q, want the part2 entry point's output", msg.stdout)
	}
}

// TestRunProgramCmdRetracesSelectedEntryForKPITabs is the feature this
// was all built for: running the Run tab's currently-selected entry
// point should also produce a fresh trace of that same entry point,
// not whichever one the session originally started with.
func TestRunProgramCmdRetracesSelectedEntryForKPITabs(t *testing.T) {
	path := writeDebugFile(t, `recipe store_part1() {
    deliver("one")
}
recipe store_part2() {
    deliver("two")
}`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.runEntryIndex = 1 // part2
	msg := m.runProgramCmd()().(runResultMsg)
	if msg.view == nil {
		t.Fatal("expected a non-nil retraced view")
	}
	if msg.store != "part2" {
		t.Errorf("store = %q, want %q", msg.store, "part2")
	}
	var buf strings.Builder
	msg.view.writePlain(&buf)
	out := buf.String()
	if !strings.Contains(out, `deliver("two")`) {
		t.Errorf("retraced view = %q, want store_part2's body traced", out)
	}
	if strings.Contains(out, `deliver("one")`) {
		t.Errorf("retraced view = %q, should not have traced store_part1's body", out)
	}
}

// TestRunProgramCmdRetraceUsesTheSameInputFile confirms the retrace
// reads the same input file the raw run does, not empty stdin -- a
// trace against no real input wouldn't reflect what actually ran.
func TestRunProgramCmdRetraceUsesTheSameInputFile(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(inputPath, []byte("42\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newDebugModel(viewFor(t, "x = 1"))
	// deliver()'s own step always shows nobox (that's deliver's return
	// value, not what it printed -- and printed output goes to
	// io.Discard for a retrace anyway) -- binding unbox()'s result to a
	// variable first is what makes the traced value itself show up as
	// a step's Out, confirming the retrace actually read the file.
	m.view.path = writeDebugFile(t, `recipe store() { line = unbox(); deliver(line) }`)
	m.runInput = runInputModel{value: []rune(inputPath)}
	msg := m.runProgramCmd()().(runResultMsg)
	if msg.view == nil {
		t.Fatal("expected a non-nil retraced view")
	}
	var buf strings.Builder
	msg.view.writePlain(&buf)
	if !strings.Contains(buf.String(), "42") {
		t.Errorf("retraced view = %q, want the input file's contents reflected", buf.String())
	}
}

// TestRunProgramCmdPersistsSelectionForNextTime confirms running from
// the Run tab remembers this file's store + input file (debug_state.go)
// so a later `crust develop` on the same file starts back here.
func TestRunProgramCmdPersistsSelectionForNextTime(t *testing.T) {
	withTempDevelStateDir(t)
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(inputPath, []byte("42\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := newDebugModel(&debugView{
		path: writeDebugFile(t, `recipe store_part1() { deliver("one") }
recipe store_part2() { deliver("two") }`),
		rec: viewFor(t, "x = 1").rec,
	})
	m.runEntryIndex = 1 // part2
	m.runInput = runInputModel{value: []rune(inputPath)}
	m.runProgramCmd()()

	got := loadDevelState()[mustAbs(t, m.view.path)]
	want := develState{Store: "part2", Input: inputPath}
	if got != want {
		t.Errorf("saved state = %+v, want %+v", got, want)
	}
}

func TestViewRunShowsEntryPointSelector(t *testing.T) {
	path := writeDebugFile(t, `recipe store_part1() { deliver(1) }
recipe store_part2() { deliver(2) }`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	out := m.viewRun()
	if !strings.Contains(out, "part1") || !strings.Contains(out, "part2") {
		t.Errorf("viewRun() = %q, want both entry points listed", out)
	}
}

func TestViewRunHidesSelectorWhenNoEntryPoints(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	out := m.viewRun()
	if strings.Contains(out, "entry point") {
		t.Errorf("viewRun() = %q, want no entry-point row for a file with none", out)
	}
}

func TestRenderEntryOptionsShowsDefaultLabel(t *testing.T) {
	out := renderEntryOptions([]string{""}, 0)
	if !strings.Contains(out, "(default)") {
		t.Errorf("renderEntryOptions([\"\"]) = %q, want it labeled (default)", out)
	}
}

func TestRunDebugTUISetsDefaultEntryIndexFromOpts(t *testing.T) {
	path := writeDebugFile(t, `recipe store_part1() { deliver(1) }
recipe store_part2() { deliver(2) }`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.opts = debugOptions{Store: "part2"}
	m.runEntryIndex = indexOfEntry(m.view.entryPoints(), m.opts.Store)
	if m.runEntryIndex != 1 {
		t.Errorf("runEntryIndex = %d, want 1 (part2)", m.runEntryIndex)
	}
}

// --- "run all stores" (Ctrl+R) ---------------------------------------------

func TestHandleRunTabKeyCtrlRTogglesRunAllStores(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabRun
	next, _ := m.handleRunTabKey(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = next.(debugModel)
	if !m.runAllStores {
		t.Fatal("expected runAllStores = true after one ctrl+r")
	}
	next, _ = m.handleRunTabKey(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = next.(debugModel)
	if m.runAllStores {
		t.Error("expected runAllStores = false after a second ctrl+r")
	}
}

func TestHandleRunTabKeyEnterDispatchesRunAllStoresCmdWhenToggled(t *testing.T) {
	path := writeDebugFile(t, `recipe store_part1() { deliver("one") }
recipe store_part2() { deliver("two") }`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.runAllStores = true
	_, cmd := m.handleRunTabKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd")
	}
	msg := cmd().(runResultMsg)
	if !strings.Contains(msg.stdout, "one") || !strings.Contains(msg.stdout, "two") {
		t.Errorf("stdout = %q, want both entry points' output", msg.stdout)
	}
}

func TestRunAllStoresCmdRunsEveryEntryPointAgainstTheSameInput(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(inputPath, []byte("42\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := writeDebugFile(t, `
recipe store_part1() { deliver("part1 saw " + unbox()) }
recipe store_part2() { deliver("part2 saw " + unbox()) }`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.runInput = runInputModel{value: []rune(inputPath)}

	msg := m.runAllStoresCmd()().(runResultMsg)
	if msg.err != nil {
		t.Fatalf("unexpected err: %v", msg.err)
	}
	if !strings.Contains(msg.stdout, "part1 saw 42") {
		t.Errorf("stdout = %q, want part1's own fresh read of the input file", msg.stdout)
	}
	if !strings.Contains(msg.stdout, "part2 saw 42") {
		t.Errorf("stdout = %q, want part2's own fresh read of the same input file", msg.stdout)
	}
}

func TestRunAllStoresCmdConcatenatesOutputWithHeadings(t *testing.T) {
	path := writeDebugFile(t, `recipe store_part1() { deliver("one") }
recipe store_part2() { deliver("two") }`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	msg := m.runAllStoresCmd()().(runResultMsg)

	partOneIdx := strings.Index(msg.stdout, "one")
	partTwoIdx := strings.Index(msg.stdout, "two")
	if partOneIdx == -1 || partTwoIdx == -1 || partOneIdx > partTwoIdx {
		t.Errorf("stdout = %q, want part1's output before part2's", msg.stdout)
	}
	if !strings.Contains(msg.stdout, "part1") || !strings.Contains(msg.stdout, "part2") {
		t.Errorf("stdout = %q, want each section headed by its own entry-point name", msg.stdout)
	}
}

func TestRunAllStoresCmdMergesTimingIntoOneView(t *testing.T) {
	path := writeDebugFile(t, `recipe store_part1() { deliver("one") }
recipe store_part2() { deliver("two") }`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	msg := m.runAllStoresCmd()().(runResultMsg)
	if msg.view == nil {
		t.Fatal("expected a non-nil merged view")
	}
	var buf strings.Builder
	msg.view.writePlain(&buf)
	out := buf.String()
	if !strings.Contains(out, `deliver("one")`) || !strings.Contains(out, `deliver("two")`) {
		t.Errorf("merged view = %q, want both stores' bodies traced", out)
	}
}

func TestRunAllStoresCmdFallsBackToBareStoreWithNoEntryPoints(t *testing.T) {
	path := writeDebugFile(t, `deliver("plain script")`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	msg := m.runAllStoresCmd()().(runResultMsg)
	if !strings.Contains(msg.stdout, "plain script") {
		t.Errorf("stdout = %q, want the plain script's output", msg.stdout)
	}
}

func TestRunAllStoresCmdReportsFailureIfAnyStoreFails(t *testing.T) {
	path := writeDebugFile(t, `recipe store_part1() { deliver("fine") }
recipe store_part2() { serve 1 / 0 }`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	msg := m.runAllStoresCmd()().(runResultMsg)
	if msg.code == 0 {
		t.Error("expected a non-zero code when one store fails")
	}
}

func TestRunAllStoresCmdMissingInputFileReturnsErr(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.view.path = writeDebugFile(t, "x = 1\n")
	m.runInput = runInputModel{value: []rune("/does/not/exist.txt")}
	msg := m.runAllStoresCmd()().(runResultMsg)
	if msg.err == nil {
		t.Fatal("expected an error for a missing input file")
	}
}

func TestRunAllStoresCmdPersistsRunAllForNextTime(t *testing.T) {
	withTempDevelStateDir(t)
	path := writeDebugFile(t, `recipe store_part1() { deliver("one") }
recipe store_part2() { deliver("two") }`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.runEntryIndex = 1 // part2
	m.runAllStoresCmd()()

	got := loadDevelState()[mustAbs(t, m.view.path)]
	want := develState{Store: "part2", Input: "", RunAll: true}
	if got != want {
		t.Errorf("saved state = %+v, want %+v", got, want)
	}
}

func TestEntryLabelDefault(t *testing.T) {
	if got := entryLabel(""); got != "(default)" {
		t.Errorf("entryLabel(\"\") = %q, want %q", got, "(default)")
	}
	if got := entryLabel("part1"); got != "part1" {
		t.Errorf("entryLabel(%q) = %q, want %q", "part1", got, "part1")
	}
}

func TestEntryFrameLabel(t *testing.T) {
	if got := entryFrameLabel(""); got != "store(...)" {
		t.Errorf("entryFrameLabel(\"\") = %q, want %q", got, "store(...)")
	}
	if got := entryFrameLabel("part1"); got != "store_part1(...)" {
		t.Errorf("entryFrameLabel(%q) = %q, want %q", "part1", got, "store_part1(...)")
	}
}

func TestViewRunShowsRunAllStoresHint(t *testing.T) {
	path := writeDebugFile(t, `recipe store_part1() { deliver(1) }
recipe store_part2() { deliver(2) }`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	out := m.viewRun()
	if !strings.Contains(out, "ctrl+r") {
		t.Errorf("viewRun() = %q, want a ctrl+r hint when entry points exist", out)
	}
}

func TestViewRunHidesRunAllStoresHintWithNoEntryPoints(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	out := m.viewRun()
	if strings.Contains(out, "ctrl+r") {
		t.Errorf("viewRun() = %q, want no ctrl+r hint for a file with no entry points", out)
	}
}

func TestViewRunShowsRunAllStoresOnState(t *testing.T) {
	path := writeDebugFile(t, `recipe store_part1() { deliver(1) }
recipe store_part2() { deliver(2) }`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.runAllStores = true
	out := m.viewRun()
	if !strings.Contains(out, "ON") {
		t.Errorf("viewRun() = %q, want the hint to show ON when runAllStores is toggled on", out)
	}
	if !strings.Contains(out, "every entry point") {
		t.Errorf("viewRun() = %q, want the pre-run instructions to describe running every entry point", out)
	}
}
