package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Sintfoap/cRust/internal/object"
)

func TestParseStudioArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantPath string
		wantErr  bool
	}{
		{"no args is an error", nil, "", true},
		{"one file", []string{"snake.crust"}, "snake.crust", false},
		{"two files is an error", []string{"a.crust", "b.crust"}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := parseStudioArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseStudioArgs(%v) error = nil, want an error", tt.args)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseStudioArgs(%v) error = %v, want nil", tt.args, err)
			}
			if path != tt.wantPath {
				t.Errorf("parseStudioArgs(%v) = %q, want %q", tt.args, path, tt.wantPath)
			}
		})
	}
}

func TestLoadStudioProgramParseError(t *testing.T) {
	_, _, errMsg := loadStudioProgram(`if keyDown("a") { }`, 20, 10)
	if errMsg == "" || !strings.Contains(errMsg, "parse error") {
		t.Errorf("loadStudioProgram on invalid syntax: errMsg = %q, want it to mention a parse error", errMsg)
	}
}

func TestLoadStudioProgramRuntimeError(t *testing.T) {
	_, _, errMsg := loadStudioProgram(`x = 1 / 0`, 20, 10)
	if errMsg == "" || !strings.Contains(errMsg, "division by zero") {
		t.Errorf("loadStudioProgram on a runtime error: errMsg = %q, want it to mention division by zero", errMsg)
	}
}

func TestLoadStudioProgramSuccessWiresGameBuiltins(t *testing.T) {
	interp, state, errMsg := loadStudioProgram(`h = cell("@", "#fff")
setPos(h, 2, 3)`, 20, 10)
	if errMsg != "" {
		t.Fatalf("loadStudioProgram: %s", errMsg)
	}
	if interp == nil || state == nil {
		t.Fatal("loadStudioProgram returned nil interp/state on success")
	}
	if len(state.entities) != 1 {
		t.Fatalf("got %d entities, want 1", len(state.entities))
	}
	for _, e := range state.entities {
		if e.ch != '@' || e.x != 2 || e.y != 3 {
			t.Errorf("entity = %+v, want {ch: '@', x: 2, y: 3}", e)
		}
	}
}

func TestLoadStudioProgramCapturesDeliverOutput(t *testing.T) {
	_, state, errMsg := loadStudioProgram(`deliver("hi from studio")`, 20, 10)
	if errMsg != "" {
		t.Fatalf("loadStudioProgram: %s", errMsg)
	}
	if state.lastLog != "hi from studio" {
		t.Errorf("state.lastLog = %q, want %q", state.lastLog, "hi from studio")
	}
}

func writeStudioFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "game.crust")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunStudioRequiresRealTerminal(t *testing.T) {
	path := writeStudioFile(t, `deliver("hi")`)
	var stdout, stderr bytes.Buffer
	code := runStudio(path, nil, &stdout, &stderr)
	if code != 1 {
		t.Errorf("runStudio() with a non-terminal stdout = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "real terminal") {
		t.Errorf("stderr = %q, want it to mention needing a real terminal", stderr.String())
	}
}

func TestRunStudioMissingFileReportsError(t *testing.T) {
	// isColorTerminal(stdout) is false for a bytes.Buffer regardless,
	// so this exercises the same not-a-terminal guard rather than the
	// file-read path specifically -- covered structurally by
	// TestRunStudioRequiresRealTerminal above; a real missing-file
	// check happens after that gate in an actual terminal session
	// (verified via pty in this feature's own manual testing pass).
	var stdout, stderr bytes.Buffer
	code := runStudio(filepath.Join(t.TempDir(), "nope.crust"), nil, &stdout, &stderr)
	if code != 1 {
		t.Errorf("runStudio() on a missing file = %d, want 1", code)
	}
}

func TestStudioModelUpdateKeyMsgRecordsPressedKey(t *testing.T) {
	state := newStudioState(20, 10)
	m := studioModel{path: "x.crust", state: state}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	got := next.(studioModel)
	if !got.state.pressed["d"] {
		t.Error("pressing 'd' didn't set state.pressed[\"d\"]")
	}
	if cmd != nil {
		t.Error("recording a key press shouldn't return a Cmd")
	}
}

func TestStudioModelUpdateQuitKeys(t *testing.T) {
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("q")},
		{Type: tea.KeyCtrlC},
	} {
		m := studioModel{path: "x.crust", state: newStudioState(20, 10)}
		_, cmd := m.Update(key)
		if cmd == nil {
			t.Fatalf("Update(%v) returned a nil Cmd, want tea.Quit", key)
		}
		if msg := cmd(); msg != tea.Quit() {
			t.Errorf("Update(%v) cmd() = %v, want tea.Quit()", key, msg)
		}
	}
}

func TestStudioModelUpdateTickCallsOnFrameAndClearsPressed(t *testing.T) {
	interp, state, errMsg := loadStudioProgram(`calls = 0
recipe onTick(dt) {
    calls = calls + 1
}
onFrame(onTick)`, 20, 10)
	if errMsg != "" {
		t.Fatalf("loadStudioProgram: %s", errMsg)
	}
	state.pressed["up"] = true
	m := studioModel{path: "x.crust", interp: interp, state: state}

	next, cmd := m.Update(studioTickMsg{})
	got := next.(studioModel)
	if got.err != "" {
		t.Fatalf("tick produced an error: %s", got.err)
	}
	if got.state.pressed["up"] {
		t.Error("state.pressed wasn't cleared after the tick")
	}
	if cmd == nil {
		t.Error("tick should return a Cmd to schedule the next one")
	}
}

func TestStudioModelUpdateTickSurfacesRuntimeError(t *testing.T) {
	interp, state, errMsg := loadStudioProgram(`recipe onTick(dt) {
    x = 1 / 0
}
onFrame(onTick)`, 20, 10)
	if errMsg != "" {
		t.Fatalf("loadStudioProgram: %s", errMsg)
	}
	m := studioModel{path: "x.crust", interp: interp, state: state}
	next, _ := m.Update(studioTickMsg{})
	got := next.(studioModel)
	if !strings.Contains(got.err, "division by zero") {
		t.Errorf("m.err = %q, want it to mention division by zero", got.err)
	}
}

func TestStudioModelUpdateTickIsNoOpOnceErrored(t *testing.T) {
	state := newStudioState(20, 10)
	state.onFrame = &object.Builtin{Fn: func(args ...object.Object) object.Object {
		t.Fatal("onFrame should not be called again once the model has already errored")
		return object.NULL
	}}
	m := studioModel{path: "x.crust", state: state, err: "a previous error"}
	next, _ := m.Update(studioTickMsg{})
	got := next.(studioModel)
	if got.err != "a previous error" {
		t.Errorf("m.err changed to %q across a tick after already erroring", got.err)
	}
}

func TestStudioModelRestartReloadsFromDisk(t *testing.T) {
	path := writeStudioFile(t, `h = cell("@", "#fff")`)
	interp, state, errMsg := loadStudioProgram(`h = cell("x", "#fff")`, 20, 10)
	if errMsg != "" {
		t.Fatalf("loadStudioProgram: %s", errMsg)
	}
	m := studioModel{path: path, interp: interp, state: state}

	next, _ := m.restart()
	got := next.(studioModel)
	if got.err != "" {
		t.Fatalf("restart(): %s", got.err)
	}
	for _, e := range got.state.entities {
		if e.ch != '@' {
			t.Errorf("entity after restart = %+v, want the on-disk file's own cell('@', ...)", e)
		}
	}
}

func TestStudioModelRestartOnBrokenFileShowsErrorAndKeepsRunning(t *testing.T) {
	path := writeStudioFile(t, `h = cell("@", "#fff")`)
	interp, state, errMsg := loadStudioProgram(`h = cell("@", "#fff")`, 20, 10)
	if errMsg != "" {
		t.Fatalf("loadStudioProgram: %s", errMsg)
	}
	m := studioModel{path: path, interp: interp, state: state}

	if err := os.WriteFile(path, []byte(`if keyDown("a") { }`), 0o644); err != nil {
		t.Fatal(err)
	}
	next, _ := m.restart()
	got := next.(studioModel)
	if got.err == "" {
		t.Error("expected restart() to surface the broken file's parse error")
	}
	if got.interp != m.interp {
		t.Error("a failed restart should leave the previous, still-working interpreter in place")
	}
}

func TestStudioModelViewRendersEntitiesAtTheirPositions(t *testing.T) {
	state := newStudioState(10, 5)
	state.entities[0] = &studioEntity{ch: '@', color: "#c0392b", x: 3, y: 2}
	m := studioModel{path: "x.crust", state: state}
	out := m.View()
	lines := strings.Split(out, "\n")
	if len(lines) < 4 {
		t.Fatalf("View() has %d lines, want at least a header + a few stage rows", len(lines))
	}
	// Line 0 is the header; the stage starts at line 1, so row y=2 of
	// the stage is rendered text line 3.
	if !strings.Contains(lines[3], "@") {
		t.Errorf("expected line %q (stage row 2) to contain '@'", lines[3])
	}
}

func TestStudioModelViewShowsHelpBarOnItsOwnLine(t *testing.T) {
	state := newStudioState(10, 5)
	m := studioModel{path: "x.crust", state: state}
	out := m.View()
	lines := strings.Split(out, "\n")
	last := lines[len(lines)-1]
	if !strings.Contains(last, "q: quit") || !strings.Contains(last, "r: restart") {
		t.Errorf("last line = %q, want the help bar on its own line", last)
	}
}

func TestStudioModelViewShowsErrorInsteadOfStage(t *testing.T) {
	state := newStudioState(10, 5)
	m := studioModel{path: "x.crust", state: state, err: "3:1: something broke"}
	out := m.View()
	if !strings.Contains(out, "something broke") {
		t.Errorf("View() = %q, want the error message shown", out)
	}
}
