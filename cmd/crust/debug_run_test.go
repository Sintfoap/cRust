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
	if m.active != tabTime {
		t.Errorf("active = %v, want tabTime (wrapped around)", m.active)
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
