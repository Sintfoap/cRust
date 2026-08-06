package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMtimeOfExistingFile(t *testing.T) {
	path := writeDebugFile(t, "x = 1\n")
	got := mtimeOf(path)
	if got.IsZero() {
		t.Error("mtimeOf(existing file) returned the zero Time")
	}
}

func TestMtimeOfMissingFileIsZero(t *testing.T) {
	if got := mtimeOf("/does/not/exist.crust"); !got.IsZero() {
		t.Errorf("mtimeOf(missing file) = %v, want the zero Time", got)
	}
}

func TestOpenEditorCmdReturnsNonNilCmd(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.view.path = writeDebugFile(t, "x = 1\n")
	if cmd := m.openEditorCmd(); cmd == nil {
		t.Error("openEditorCmd() returned a nil Cmd")
	}
}

func TestHandleKeyEnterOnEditorTabReturnsCmd(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.view.path = writeDebugFile(t, "x = 1\n")
	m.active = tabEditor
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("enter on the Editor tab should return a non-nil Cmd (openEditorCmd)")
	}
}

func TestHandleNvimExitErrorSetsEditorErr(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	next, cmd := m.handleNvimExit(nvimExitMsg{err: errors.New("exec: \"nvim\": executable file not found in $PATH")})
	m = next.(debugModel)
	if m.editorErr == "" {
		t.Error("expected editorErr to be set after a failed nvim launch")
	}
	if cmd != nil {
		t.Error("a launch failure should not trigger a reload")
	}
}

func TestHandleNvimExitCleanQuitWithoutSaveIsNoOp(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.editorErr = "stale error from a previous attempt"
	next, cmd := m.handleNvimExit(nvimExitMsg{err: nil, saved: false})
	m = next.(debugModel)
	if m.editorErr != "" {
		t.Errorf("editorErr = %q, want it cleared on a clean exit", m.editorErr)
	}
	if cmd != nil {
		t.Error("quitting without saving should not trigger a reload")
	}
}

func TestHandleNvimExitSavedTriggersReload(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.view.path = writeDebugFile(t, "x = 1\n")
	next, cmd := m.handleNvimExit(nvimExitMsg{err: nil, saved: true})
	_ = next.(debugModel)
	if cmd == nil {
		t.Error("a save should trigger reloadCmd")
	}
}

func TestReloadCmdSuccessProducesNewView(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.view.path = writeDebugFile(t, "x = 1\ny = 2\n")
	msg := m.reloadCmd()().(reloadMsg)
	if msg.err != nil {
		t.Fatalf("reloadCmd() error = %v, want nil", msg.err)
	}
	if msg.view == nil {
		t.Fatal("reloadCmd() returned a nil view on success")
	}
	if msg.view.rec.Steps() != 2 {
		t.Errorf("reloaded recording has %d steps, want 2", msg.view.rec.Steps())
	}
}

func TestReloadCmdParseErrorReturnsErr(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.view.path = writeDebugFile(t, "x = (\n")
	msg := m.reloadCmd()().(reloadMsg)
	if msg.err == nil {
		t.Fatal("reloadCmd() on a file with a parse error should return err")
	}
}

// TestReloadCmdUsesRunTabInputFile confirms reloadCmd reads its stdin
// from m.runInput (the Run tab's own field) rather than real process
// stdin -- the same fix runProgramCmd's retrace already applies,
// closing the same "eats the user's keystrokes as unbox() input"
// class of bug for the Editor tab's save-triggered reload.
func TestReloadCmdUsesRunTabInputFile(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(inputPath, []byte("42\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := newDebugModel(viewFor(t, "x = 1"))
	m.view.path = writeDebugFile(t, `line = unbox(); deliver(line)`)
	m.runInput = runInputModel{value: []rune(inputPath)}

	msg := m.reloadCmd()().(reloadMsg)
	if msg.err != nil {
		t.Fatalf("reloadCmd() error = %v, want nil", msg.err)
	}
	var buf strings.Builder
	msg.view.writePlain(&buf)
	if !strings.Contains(buf.String(), "42") {
		t.Errorf("reloaded view = %q, want the Run tab's input file reflected", buf.String())
	}
}

// TestReloadCmdUnreadableInputFileDegradesToEmpty confirms a stale or
// missing input path doesn't block the reload -- an edit unrelated to
// the input file should still refresh the recording, just with empty
// stdin, the same as if no input file had ever been given.
func TestReloadCmdUnreadableInputFileDegradesToEmpty(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.view.path = writeDebugFile(t, `deliver("hello")`)
	m.runInput = runInputModel{value: []rune("/no/such/input.txt")}

	msg := m.reloadCmd()().(reloadMsg)
	if msg.err != nil {
		t.Fatalf("reloadCmd() error = %v, want nil (missing input file should degrade, not fail)", msg.err)
	}
	if msg.view == nil {
		t.Fatal("reloadCmd() returned a nil view")
	}
}

// TestReloadCmdNoInputFileStillRecordsARun confirms the common case
// (no input file ever set in the Run tab) still produces a real
// recording rather than erroring or hanging -- reloadCmd no longer
// references any process-stdin-derived reader at all in this case
// (bytes.NewReader(nil), not os.Stdin), which is what actually
// guarantees a program calling unbox() here can't consume real
// keystrokes; this just confirms that path still works end to end.
func TestReloadCmdNoInputFileStillRecordsARun(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.view.path = writeDebugFile(t, `line = unbox(); deliver(slices(line))`)

	msg := m.reloadCmd()().(reloadMsg)
	if msg.err != nil {
		t.Fatalf("reloadCmd() error = %v, want nil", msg.err)
	}
	if msg.view.rec.Steps() == 0 {
		t.Error("expected a real recording (nonzero steps), not an empty one")
	}
}

func TestReloadCmdDiscardsProgramOutput(t *testing.T) {
	// A reload's interpreter must never write straight to the real
	// terminal (it would corrupt the already-active alt-screen), so
	// there's no stdout to assert on here beyond "this doesn't panic
	// or hang" -- deliver() output goes to io.Discard by construction.
	m := newDebugModel(viewFor(t, "x = 1"))
	m.view.path = writeDebugFile(t, `deliver("hello")`)
	msg := m.reloadCmd()().(reloadMsg)
	if msg.err != nil {
		t.Fatalf("reloadCmd() error = %v, want nil", msg.err)
	}
}

func TestHandleReloadSuccessResetsStepperState(t *testing.T) {
	m := newDebugModel(viewFor(t, `
total = 0
knead n in [1, 2, 3, 4, 5] {
    total += n
}
`))
	m.active = tabEditor
	m.cursor = 3
	m.editorErr = "stale"

	newView := viewFor(t, "x = 1")
	next, cmd := m.handleReload(reloadMsg{view: newView})
	m = next.(debugModel)
	if m.view != newView {
		t.Error("handleReload should replace the view on success")
	}
	if m.cursor != 0 || m.top != 0 {
		t.Errorf("cursor, top = %d, %d, want 0, 0 after a reload", m.cursor, m.top)
	}
	if len(m.expanded) != 0 {
		t.Error("expanded folds should reset on a reload")
	}
	if m.editorErr != "" {
		t.Errorf("editorErr = %q, want cleared on a successful reload", m.editorErr)
	}
	if m.active != tabTime {
		t.Errorf("active = %v, want tabTime after a successful reload (back to the dashboard)", m.active)
	}
	if cmd != nil {
		t.Error("a successful reload should not reopen nvim -- the user already chose to quit")
	}
}

func TestHandleReloadFailureKeepsOldView(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabEditor
	original := m.view
	next, cmd := m.handleReload(reloadMsg{err: errors.New("1 parse error(s):\n  bad token")})
	m = next.(debugModel)
	if m.view != original {
		t.Error("handleReload should keep the previous view when the reload failed")
	}
	if m.editorErr == "" {
		t.Error("expected editorErr to report the reload failure")
	}
	if m.active != tabEditor {
		t.Errorf("active = %v, want tabEditor so the error stays visible", m.active)
	}
	if cmd != nil {
		t.Error("a failed reload should not reopen nvim automatically -- the user already chose to quit")
	}
}

func TestViewEditorShowsPathAndInstructions(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.view.path = "myprog.crust"
	out := m.viewEditor()
	if !strings.Contains(out, "myprog.crust") {
		t.Errorf("viewEditor() = %q, want it to mention the file path", out)
	}
	if !strings.Contains(out, "nvim") {
		t.Errorf("viewEditor() = %q, want instructions mentioning nvim", out)
	}
}

func TestViewEditorShowsLastError(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.editorErr = "executable file not found in $PATH"
	out := m.viewEditor()
	if !strings.Contains(out, "executable file not found in $PATH") {
		t.Errorf("viewEditor() = %q, want the last error surfaced", out)
	}
}

func TestHelpTextForEditorTab(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabEditor
	if got := m.helpText(); !strings.Contains(got, "nvim") {
		t.Errorf("helpText() = %q, want it to mention nvim", got)
	}
}

func TestDebugModelUpdateDispatchesNvimExitMsg(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	next, _ := m.Update(nvimExitMsg{err: errors.New("boom")})
	if got := next.(debugModel).editorErr; got == "" {
		t.Error("Update() should route nvimExitMsg to handleNvimExit")
	}
}

func TestDebugModelUpdateDispatchesReloadMsg(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	newView := viewFor(t, "y = 2")
	next, _ := m.Update(reloadMsg{view: newView})
	if next.(debugModel).view != newView {
		t.Error("Update() should route reloadMsg to handleReload")
	}
}

func TestDebugModelViewEditorTabRendersFile(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.view.path = "some/file.crust"
	m.active = tabEditor
	m.width, m.height = 100, 30
	out := m.View()
	if !strings.Contains(out, "some/file.crust") {
		t.Errorf("View() on the Editor tab = %q, want the file path", out)
	}
}
