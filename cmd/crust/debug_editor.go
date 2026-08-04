// The Editor tab's nvim hand-off (debug_tui.go's tabEditor). `crust
// debug` hands the whole terminal to a real nvim via tea.ExecProcess
// rather than drawing an editor pane inline — embedding one would mean
// shipping a full terminal emulator inside this TUI just to render
// nvim's own screen, a much bigger and more fragile build than reusing
// a real editor wholesale (see ARCHITECTURE.md's debugger section for
// the fuller tradeoff). An autocmd makes nvim quit the instant the
// buffer is saved, and the callback below reopens it automatically
// after rerunning the recording — so from the user's chair it reads as
// "save and the debugger updates," even though under the hood each
// save is really its own nvim process handing control straight back.
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

// openEditorCmd hands the terminal to nvim on m.view.path, preloaded
// with an autocmd that quits it the moment the buffer is saved. mtime,
// captured before nvim runs, is how the callback tells "saved and
// quit" apart from "quit without saving" — nvim's own exit status is 0
// either way, so it can't be the signal, and comparing file content
// would mean reading a potentially large file twice for no more
// certainty than the mtime already gives.
func (m debugModel) openEditorCmd() tea.Cmd {
	path := m.view.path
	before := mtimeOf(path)

	cmd := exec.Command("nvim", "-c", "autocmd BufWritePost <buffer> quitall!", path)
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

// handleNvimExit reacts to nvim handing the terminal back. A launch
// failure (nvim missing, a bad exit) surfaces on the Editor tab and
// goes no further; a clean quit with no save just returns to the
// Editor tab as it was; a save kicks off reloadCmd, which — on success
// — reopens nvim itself, so the loop continues without the user
// needing to press enter again.
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
// doesn't blank out the KPI/Stepper tabs. Success replaces the view and
// resets the stepper's fold/cursor state, since the new recording's
// tree has no relationship to the old one's, then reopens nvim to
// continue editing.
func (m debugModel) handleReload(msg reloadMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.editorErr = msg.err.Error()
		return m, m.openEditorCmd()
	}
	m.editorErr = ""
	m.view = msg.view
	m.expanded = map[*debugger.TraceNode]bool{}
	m.cursor, m.top = 0, 0
	m.rebuildRows()
	return m, m.openEditorCmd()
}
