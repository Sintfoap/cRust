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
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"
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

// nvimCmd builds the *exec.Cmd that opens nvim on path, split out from
// openEditorCmd purely so cmd.Stdin's value is directly assertable by
// a Go test rather than only observable through a real pty session.
//
// cmd.Stdin is set explicitly to the real os.Stdin here, ahead of
// tea.ExecProcess -- bubbletea's own ExecProcess only fills in Stdin
// when it's still unset (osExecCommand.SetStdin's "if c.Stdin == nil"
// guard), and would otherwise hand nvim its own wrapped p.input
// (debug_tui.go's stdinNoNamer), which is deliberately not a real
// *os.File so cancelreader/term.File's own type assertions treat it
// right (see stdinNoNamer's doc comment). That's exactly the problem
// for a child *process*, though: since it isn't a concrete *os.File,
// Go's os/exec can't hand its fd to nvim directly and instead opens an
// os.Pipe and spins up a background goroutine to copy bytes from it --
// a real, if intermittent, bug in practice: a real pty-driven session
// reproduced nvim exiting cleanly (0) after a genuine :wq save, but
// with that copier goroutine's own Read on the real terminal fd racing
// the exit and returning a transient EAGAIN, which os/exec then
// surfaces as cmd.Run()'s own error even though nvim itself never saw
// it -- and openEditorCmd's caller (handleNvimExit) treats *any*
// non-nil err as a hard launch/exit failure, stopping there instead of
// reaching the entry-point rescan or the save-triggered reload, so a
// perfectly good save silently never refreshed the Run tab. Presetting
// a real *os.File sidesteps the whole pipe-and-goroutine path: os/exec
// dup's the fd straight into the child, the same as any ordinary
// shell's redirection would, with nothing left to race.
func nvimCmd(path string) *exec.Cmd {
	cmd := exec.Command("nvim", path)
	cmd.Stdin = os.Stdin
	return cmd
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

	cmd := nvimCmd(path)
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
// Editor tab and goes no further. Otherwise the Run tab's entry-point
// list is refreshed unconditionally — not only when msg.saved says a
// save happened — since that's a coarse mtime-diff heuristic (a save
// within the same mtime-resolution window as the file was opened would
// slip past it) and rescanning is cheap: a lex+parse, not a re-trace.
// A save additionally kicks off reloadCmd, whose result (handleReload)
// is what actually leaves the Editor tab and re-records the file — a
// clean exit with nothing saved has no reason to do that, since the
// existing recording is still accurate. nvim itself is never reopened
// automatically here, since the only way this fires at all is the user
// having already chosen to quit.
func (m debugModel) handleNvimExit(msg nvimExitMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.editorErr = msg.err.Error()
		return m, nil
	}
	m.editorErr = ""
	m.view.refreshEntryPoints()
	m.runEntryIndex = indexOfEntry(m.view.entryPoints(), m.opts.Store)
	if !msg.saved {
		return m, nil
	}
	return m, m.reloadCmd()
}

// reloadCmd re-parses and re-records m.view.path with the same options
// the current recording used. Its interpreter's program output
// (deliver, etc.) goes to io.Discard rather than the real terminal —
// unlike --plain's eager run (built before the TUI ever took the
// screen), this one runs while the alt-screen buffer is already
// active, and writing straight to the terminal here would corrupt it.
//
// Reads stdin from the Run tab's own input-file field (m.runInput),
// the same source runProgramCmd uses for an explicit run — never real
// process stdin, which the interactive TUI doesn't touch for tracing
// at all (see emptyDebugView in debug.go for why). An unreadable input
// path (e.g. moved since it was last used) degrades to empty input
// here rather than failing the whole reload: an edit that has nothing
// to do with the input file shouldn't be blocked by a stale path — the
// Run tab's own "run" action still reports that error loudly when it's
// actually the thing being run.
func (m debugModel) reloadCmd() tea.Cmd {
	path, opts := m.view.path, m.opts
	inputPath := strings.TrimSpace(m.runInput.String())
	return func() tea.Msg {
		var data []byte
		if inputPath != "" {
			if d, err := os.ReadFile(inputPath); err == nil {
				data = d
			}
		}
		view, err := buildDebugView(path, opts, bytes.NewReader(data), io.Discard)
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
// no relationship to the old one's) and the Run tab's entry-point
// selection (the edit could have added, removed, or renamed
// store/store_<name> recipes), and switches to the Time tab — the save
// is done and nvim already closed, so landing back on the dashboard is
// the point, not another editing pass.
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
	m.runEntryIndex = indexOfEntry(m.view.entryPoints(), m.opts.Store)
	m.runEntryFocused = false
	m.active = tabTime
	return m, nil
}
