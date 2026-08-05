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

// handleRunTabKey routes keys on the Run tab. Which row has focus
// (m.runEntryFocused) decides where most keys go: Left/Right and typed
// characters mean "edit the input-file field" when it has focus, or
// "cycle the entry-point selector" when that row does instead. Almost
// everything typed while the field has focus goes straight to it,
// including letters bound to actions on every other tab (q for quit,
// h/j/k/l for movement), since a file path can legitimately contain
// any of them. Only the handful of keys a path could never need stay
// reserved: Up/Down to move focus between the two rows, Tab/Shift+Tab
// to switch tabs, Enter to run, and Ctrl+C/Esc as an always-available
// way out that doesn't depend on typing a bare "q".
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
	case tea.KeyUp, tea.KeyDown:
		m.toggleRunFocus()
	case tea.KeyLeft:
		if m.runEntryFocused {
			m.cycleRunEntry(-1)
		} else {
			m.runInput.left()
		}
	case tea.KeyRight:
		if m.runEntryFocused {
			m.cycleRunEntry(1)
		} else {
			m.runInput.right()
		}
	case tea.KeyRunes:
		if !m.runEntryFocused {
			for _, r := range msg.Runes {
				m.runInput.insert(r)
			}
		}
	case tea.KeySpace:
		if !m.runEntryFocused {
			m.runInput.insert(' ')
		}
	case tea.KeyBackspace:
		if !m.runEntryFocused {
			m.runInput.backspace()
		}
	case tea.KeyDelete:
		if !m.runEntryFocused {
			m.runInput.deleteForward()
		}
	case tea.KeyHome, tea.KeyCtrlA:
		if !m.runEntryFocused {
			m.runInput.cursor = 0
		}
	case tea.KeyEnd, tea.KeyCtrlE:
		if !m.runEntryFocused {
			m.runInput.cursor = len(m.runInput.value)
		}
	}
	return m, nil
}

// toggleRunFocus switches between the input-file field and the
// entry-point selector, when there's actually a selector to switch
// to — a file with no store/store_<name> recipes at all (the common
// case for a small top-to-bottom AoC script) has nothing to select, so
// Up/Down stays a no-op and focus stays on the field.
func (m *debugModel) toggleRunFocus() {
	if len(m.runEntryOptions()) == 0 {
		return
	}
	m.runEntryFocused = !m.runEntryFocused
}

// cycleRunEntry moves the selected entry point by delta, wrapping —
// a small, fixed-size row of choices, so wraparound (like the tab
// bar's own) reads better than clamping at the ends.
func (m *debugModel) cycleRunEntry(delta int) {
	n := len(m.runEntryOptions())
	if n == 0 {
		return
	}
	m.runEntryIndex = ((m.runEntryIndex+delta)%n + n) % n
}

// runEntryOptions returns the file's store/store_<name> entry points
// (debugView.entryPoints, scanned once and cached there per view), for
// the Run tab's selector.
func (m debugModel) runEntryOptions() []string {
	return m.view.entryPoints()
}

// selectedRunEntry returns the currently selected entry point's
// --store value ("" for the bare `store`), or "" when the file has no
// entry points to select from — runFile already treats an empty store
// the same way `crust run` with no --store flag does.
func (m debugModel) selectedRunEntry() string {
	options := m.runEntryOptions()
	if len(options) == 0 {
		return ""
	}
	i := m.runEntryIndex
	if i < 0 || i >= len(options) {
		i = 0
	}
	return options[i]
}

// indexOfEntry returns store's position among options ("" for the bare
// `store`, same convention collectEntryPoints uses), or 0 if store
// isn't among them (including when options is empty) — 0 is always a
// vacuous-but-safe default, since runEntryOptions()/selectedRunEntry()
// only ever consult it when len(options) > 0.
func indexOfEntry(options []string, store string) int {
	for i, o := range options {
		if o == store {
			return i
		}
	}
	return 0
}

// runResultMsg carries one Run-tab execution's result.
type runResultMsg struct {
	stdout string
	stderr string
	code   int
	err    error // couldn't open the given input file; distinct from a program-level failure, which shows up in stderr/code instead
}

// runProgramCmd runs m.view.path exactly the way `crust run` would —
// same runFile (run.go), with whichever entry point the Run tab's
// selector currently has picked — with the Run tab's input-file path
// (if any) opened as stdin. An empty path means no stdin at all, same
// as running with input redirected from /dev/null.
func (m debugModel) runProgramCmd() tea.Cmd {
	path, store := m.view.path, m.selectedRunEntry()
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

// viewRun is the Run tab: the input-file field, the entry-point
// selector (only when the file actually has store/store_<name> recipes
// to choose from), then either instructions (nothing has run yet) or
// the last run's output. A "> " marker in front of whichever row has
// focus mirrors the Stepper's own cursor-row convention, rather than
// inventing a second way to show "this is the active one."
func (m debugModel) viewRun() string {
	var b strings.Builder

	inputMarker := "  "
	if !m.runEntryFocused {
		inputMarker = "> "
	}
	b.WriteString(inputMarker + styleTitle.Render("input file (optional, becomes stdin):"))
	b.WriteByte('\n')
	b.WriteString("  " + m.runInput.render())
	b.WriteString("\n\n")

	if options := m.runEntryOptions(); len(options) > 0 {
		entryMarker := "  "
		if m.runEntryFocused {
			entryMarker = "> "
		}
		b.WriteString(entryMarker + styleTitle.Render("entry point (↑↓ to select this row, ←→ to change):"))
		b.WriteByte('\n')
		b.WriteString("  " + renderEntryOptions(options, m.runEntryIndex))
		b.WriteString("\n\n")
	}

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

// renderEntryOptions draws the entry-point selector as a row of
// labels with the selected one highlighted — the same "row of choices,
// one picked out" shape the tab bar itself uses, just inline rather
// than across the top of the screen. "" (the bare `store`) renders as
// "(default)", matching --store's own convention that omitting it
// means the bare store recipe.
func renderEntryOptions(options []string, selected int) string {
	parts := make([]string, len(options))
	for i, o := range options {
		label := o
		if label == "" {
			label = "(default)"
		}
		if i == selected {
			parts[i] = styleSelectedRow.Render(label)
		} else {
			parts[i] = styleMuted.Render(label)
		}
	}
	return strings.Join(parts, "   ")
}
