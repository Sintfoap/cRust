// The Editor tab's nvim hand-off (debug_tui.go's tabEditor). `crust
// debug` hands the whole terminal to a real nvim via tea.ExecProcess
// rather than drawing an editor pane inline — embedding one would mean
// shipping a full terminal emulator inside this TUI just to render
// nvim's own screen, a much bigger and more fragile build than reusing
// a real editor wholesale (see ARCHITECTURE.md's debugger section for
// the fuller tradeoff). nvim runs completely natively — no autocmd
// forcing it to quit on every save — so :w saves and keeps editing
// exactly like it would anywhere else; the debugger only regains
// control (and only then rechecks the file) once nvim actually exits,
// however the user chose to do that (:wq, :x, ZZ, or plain :q).
package main

import (
	"io"
	"os"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Sintfoap/cRust/internal/debugger"
)

// nvimExitMsg reports how one hand-off to nvim ended.
type nvimExitMsg struct {
	err   error
	saved bool
}

// reloadMsg carries the result of re-recording the debugged file after
// a save, for the Editor tab's refresh.
type reloadMsg struct {
	view *debugView
	err  error
}

// openEditorCmd hands the terminal to nvim on m.view.path. mtime,
// captured before nvim runs, is how the callback tells "the file
// changed during this session" apart from "nothing was saved" once
// nvim exits — nvim's own exit status is 0 whether the user quit via
// :wq or a plain :q, so it can't be the signal, and comparing file
// content would mean reading a potentially large file twice for no
// more certainty than the mtime already gives.
func (m debugModel) openEditorCmd() tea.Cmd {
	path := m.view.path
	before := mtimeOf(path)

	cmd := exec.Command("nvim", path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return nvimExitMsg{err: err, saved: err == nil && mtimeOf(path) != before}
	})
}

// mtimeOf returns path's modification time, or the zero Time if it
// can't be read — good enough for a not-equal comparison either way,
// and a stat failure here shouldn't be fatal to the debugger.
func mtimeOf(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// handleNvimExit reacts to nvim handing the terminal back, however it
// exited. A launch failure (nvim missing, a bad exit) surfaces on the
// Editor tab and goes no further; quitting with nothing saved is a
// no-op, since there's nothing new to show; a save kicks off
// reloadCmd, whose result (handleReload) is what actually leaves the
// Editor tab — nvim itself is never reopened automatically here, since
// the only way this fires at all is the user having already chosen to
// quit.
func (m debugModel) handleNvimExit(msg nvimExitMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.editorErr = msg.err.Error()
		return m, nil
	}
	m.editorErr = ""
	if !msg.saved {
		return m, nil
	}
	return m, m.reloadCmd()
}

// reloadCmd re-parses and re-records m.view.path with the same options
// the current recording used. Its interpreter's program output
// (deliver, etc.) goes to io.Discard rather than the real terminal —
// unlike the very first recording (built before the TUI ever took the
// screen), this one runs while the alt-screen buffer is already
// active, and writing straight to the terminal here would corrupt it.
func (m debugModel) reloadCmd() tea.Cmd {
	path, opts, stdin := m.view.path, m.opts, m.stdin
	return func() tea.Msg {
		view, err := buildDebugView(path, opts, stdin, io.Discard)
		if err != nil {
			return reloadMsg{err: err}
		}
		return reloadMsg{view: view}
	}
}

// handleReload applies a reload's result. Failure (e.g. a save that
// left the file with a parse error) is shown on the Editor tab without
// touching the previous — still valid — recording, so a typo mid-edit
// doesn't blank out the Time/Memory/Stepper tabs; the user already
// chose to quit nvim, so this leaves them on the Editor tab to read the
// error and reopen it themselves (enter, or tab away and back) rather
// than forcing them straight back in. Success replaces the view,
// resets the stepper's fold/cursor state (the new recording's tree has
// no relationship to the old one's), and switches to the Time tab —
// the save is done and nvim already closed, so landing back on the
// dashboard is the point, not another editing pass.
func (m debugModel) handleReload(msg reloadMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.editorErr = msg.err.Error()
		return m, nil
	}
	m.editorErr = ""
	m.view = msg.view
	m.expanded = map[*debugger.TraceNode]bool{}
	m.cursor, m.top = 0, 0
	m.rebuildRows()
	m.active = tabTime
	return m, nil
}
