// The Run tab (debug_tui.go's tabRun): type a path to an input file
// and press enter to run the file being debugged with it as stdin,
// showing the raw output — exactly what `crust run file.crust <
// input.txt` would print, not the structured per-statement trace the
// KPI/Stepper tabs show. It reuses runFile (run.go) directly rather
// than going through debugger.Recorder at all, so this tab's output is
// genuinely "the command line," with no tracing overhead or KPI
// bucketing in the way.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// runInputModel is a minimal single-line text field for the input-file
// path — hand-rolled rather than reaching for a components library
// (e.g. charmbracelet/bubbles' textinput): inserting/deleting runes
// around a cursor is a small enough job that a third TUI dependency,
// after bubbletea + lipgloss, isn't worth it under this project's
// add-a-dependency-only-when-needed policy.
type runInputModel struct {
	value  []rune
	cursor int
}

func (in *runInputModel) insert(r rune) {
	in.value = append(in.value[:in.cursor:in.cursor], append([]rune{r}, in.value[in.cursor:]...)...)
	in.cursor++
}

func (in *runInputModel) backspace() {
	if in.cursor == 0 {
		return
	}
	in.value = append(in.value[:in.cursor-1], in.value[in.cursor:]...)
	in.cursor--
}

func (in *runInputModel) deleteForward() {
	if in.cursor >= len(in.value) {
		return
	}
	in.value = append(in.value[:in.cursor], in.value[in.cursor+1:]...)
}

func (in *runInputModel) left() {
	if in.cursor > 0 {
		in.cursor--
	}
}

func (in *runInputModel) right() {
	if in.cursor < len(in.value) {
		in.cursor++
	}
}

func (in runInputModel) String() string {
	return string(in.value)
}

// render draws the field with a reverse-video block standing in for a
// terminal cursor at the current position — a plain lipgloss.Reverse
// on one rune (or a trailing space, past the last rune) reads as a
// cursor regardless of the surrounding theme.
func (in runInputModel) render() string {
	if in.cursor >= len(in.value) {
		return string(in.value) + styleCursor.Render(" ")
	}
	before := string(in.value[:in.cursor])
	at := styleCursor.Render(string(in.value[in.cursor]))
	after := string(in.value[in.cursor+1:])
	return before + at + after
}

// handleRunTabKey routes keys while the Run tab's input field has
// focus. Almost everything typed goes straight to the field, including
// letters bound to actions on every other tab (q for quit, h/j/k/l for
// movement), since a file path can legitimately contain any of them.
// Only the handful of keys a path could never need stay reserved: Tab/
// Shift+Tab to switch tabs, Enter to run, and Ctrl+C/Esc as an
// always-available way out that doesn't depend on typing a bare "q".
func (m debugModel) handleRunTabKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyTab:
		m.active = (m.active + 1) % tabCount
		return m, m.maybeOpenEditor()
	case tea.KeyShiftTab:
		m.active = (m.active + tabCount - 1) % tabCount
		return m, m.maybeOpenEditor()
	case tea.KeyEnter:
		return m, m.runProgramCmd()
	case tea.KeyCtrlC, tea.KeyEsc:
		return m, tea.Quit
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			m.runInput.insert(r)
		}
	case tea.KeySpace:
		m.runInput.insert(' ')
	case tea.KeyBackspace:
		m.runInput.backspace()
	case tea.KeyDelete:
		m.runInput.deleteForward()
	case tea.KeyLeft:
		m.runInput.left()
	case tea.KeyRight:
		m.runInput.right()
	case tea.KeyHome, tea.KeyCtrlA:
		m.runInput.cursor = 0
	case tea.KeyEnd, tea.KeyCtrlE:
		m.runInput.cursor = len(m.runInput.value)
	}
	return m, nil
}

// runResultMsg carries one Run-tab execution's result.
type runResultMsg struct {
	stdout string
	stderr string
	code   int
	err    error // couldn't open the given input file; distinct from a program-level failure, which shows up in stderr/code instead
}

// runProgramCmd runs m.view.path exactly the way `crust run` would —
// same runFile (run.go), same --store — with the Run tab's input-file
// path (if any) opened as stdin. An empty path means no stdin at all,
// same as running with input redirected from /dev/null.
func (m debugModel) runProgramCmd() tea.Cmd {
	path, store := m.view.path, m.opts.Store
	inputPath := strings.TrimSpace(m.runInput.String())
	return func() tea.Msg {
		stdin := io.Reader(strings.NewReader(""))
		if inputPath != "" {
			f, err := os.Open(inputPath)
			if err != nil {
				return runResultMsg{err: err}
			}
			defer f.Close()
			stdin = f
		}
		var stdout, stderr bytes.Buffer
		code := runFile(path, store, stdin, &stdout, &stderr)
		return runResultMsg{stdout: stdout.String(), stderr: stderr.String(), code: code}
	}
}

// handleRunResult applies one Run-tab execution's result to the model.
// stdout and stderr are shown concatenated, in that order, matching
// what a real terminal running the same command would show as long as
// the program doesn't interleave them faster than its own buffering —
// good enough for "what did this do", which is the question this tab
// answers.
func (m debugModel) handleRunResult(msg runResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.runOutput = msg.err.Error()
		m.runFailed = true
		return m, nil
	}
	out := msg.stdout
	if msg.stderr != "" {
		if out != "" && !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		out += msg.stderr
	}
	if out == "" {
		out = "(no output)"
	}
	m.runOutput = out
	m.runFailed = msg.code != 0
	return m, nil
}

// viewRun is the Run tab: the input-file field, then either
// instructions (nothing has run yet) or the last run's output.
func (m debugModel) viewRun() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("input file (optional, becomes stdin):"))
	b.WriteByte('\n')
	b.WriteString(m.runInput.render())
	b.WriteString("\n\n")
	if m.runOutput == "" {
		b.WriteString(styleMuted.Render(fmt.Sprintf(
			"enter: run %s (with the path above as stdin, if any) and show its output here, like running it from the command line",
			m.view.path)))
	} else {
		style := lipgloss.NewStyle()
		if m.runFailed {
			style = styleError
		}
		b.WriteString(style.Render(m.runOutput))
	}
	return b.String()
}
