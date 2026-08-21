// `crust studio`'s bubbletea Model -- the terminal-native counterpart
// to `crust game`'s browser+WASM runtime (game.go, cmd/wasmgame). Same
// underlying idea (a persistent interpreter, a per-frame callback,
// spawn/move/destroy handles) driven by a fundamentally different host:
// no browser, no WASM boundary, no PixiJS -- this runs as an ordinary
// part of the `crust` binary, rendering directly into the terminal via
// lipgloss the same way every other `crust develop` tab already does.
package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Sintfoap/cRust/internal/interpreter"
	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/object"
	"github.com/Sintfoap/cRust/internal/parser"
)

// studioTickInterval is how often onFrame(dt) is called. 15Hz rather
// than the browser's 60fps rAF -- a terminal redraw is a full-screen
// text repaint, not a compositor blit, and a real-time terminal game's
// own "smooth enough" bar is a lot lower than a browser canvas's. It
// also sets keyDown()'s own effective resolution: see that builtin's
// doc comment (studio_builtins.go) for why a slower tick gives a
// tapped key more room to land inside one frame's window.
const studioTickInterval = time.Second / 15

type studioTickMsg time.Time

func studioTickCmd() tea.Cmd {
	return tea.Tick(studioTickInterval, func(t time.Time) tea.Msg { return studioTickMsg(t) })
}

// studioMinCols/studioMinRows are the smallest playable stage this
// tool will ever report, regardless of what the real terminal (or a
// misconfigured/non-negotiating one -- some SSH sessions, some
// multiplexer configurations, a raw pty with no TIOCSWINSZ ever sent)
// claims its size is. A degenerate 0x0 or 1x1 stageSize() wouldn't
// just look bad, it would make an ordinary game (a border at the
// stage's own edges, bounds-checked movement) fail its own bounds
// check on the very first tick and appear to end instantly with no
// visible cause -- worth guarding against explicitly rather than
// trusting the terminal always reports something sane.
const (
	studioMinCols = 20
	studioMinRows = 8
)

// studioStageDims reserves one line each for the header and help bar,
// leaving the rest of the real terminal as the game's own stageSize().
func studioStageDims(width, height int) (cols, rows int) {
	cols = width
	rows = height - 2
	if cols < studioMinCols {
		cols = studioMinCols
	}
	if rows < studioMinRows {
		rows = studioMinRows
	}
	return cols, rows
}

// studioLogWriter is deliver()'s destination inside `crust studio`.
// There's no separate console panel the way crust game's browser page
// has room for -- this is a full-screen terminal repaint, not a
// two-pane layout -- so only the most recent line is kept and shown on
// the help bar; still enough to glance at a value while iterating,
// just not a scrollback.
type studioLogWriter struct{ state *studioState }

func (w studioLogWriter) Write(p []byte) (int, error) {
	w.state.lastLog = strings.TrimRight(string(p), "\n")
	return len(p), nil
}

// loadStudioProgram lexes, parses, and evaluates src's top-level code
// (there's no store/store_<name> entry point here, same as crust
// game -- top-level code doubles as setup) against a fresh interpreter
// with the studio builtin table merged in. Shared between the initial
// load (studio.go's runStudio) and every `r` restart
// (studioModel.restart below), so the two can never drift on how a
// program actually gets loaded.
func loadStudioProgram(src string, cols, rows int) (*interpreter.Interpreter, *studioState, string) {
	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return nil, nil, fmt.Sprintf("%d parse error(s):\n  %s", len(errs), strings.Join(errs, "\n  "))
	}

	state := newStudioState(cols, rows)
	interp := interpreter.New(studioLogWriter{state}, strings.NewReader(""))
	for name, b := range studioBuiltins(state) {
		interp.Builtins[name] = b
	}
	env := object.NewEnvironment()

	result := interp.Eval(program, env)
	if errObj, ok := result.(*object.Error); ok {
		return nil, nil, fmt.Sprintf("%d:%d: %s", errObj.Line, errObj.Col, errObj.Message)
	}
	return interp, state, ""
}

type studioModel struct {
	path   string
	interp *interpreter.Interpreter
	state  *studioState

	// err holds the last parse/runtime error, from either the initial
	// load or a later `r` restart/tick -- the stage stops rendering
	// and onFrame stops being called while this is set, the same
	// "freeze rather than crash the whole tool" posture a bad program
	// gets everywhere else in crust develop.
	err string

	width, height int
}

func (m studioModel) Init() tea.Cmd {
	return studioTickCmd()
}

func (m studioModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.state.cols, m.state.rows = studioStageDims(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "r":
			return m.restart()
		default:
			m.state.pressed[msg.String()] = true
			return m, nil
		}

	case studioTickMsg:
		if m.err == "" && m.state.onFrame != nil {
			result := m.interp.Call(m.state.onFrame, []object.Object{&object.Float{Value: studioTickInterval.Seconds()}})
			if errObj, ok := result.(*object.Error); ok {
				m.err = fmt.Sprintf("%d:%d: %s", errObj.Line, errObj.Col, errObj.Message)
			}
		}
		for k := range m.state.pressed {
			delete(m.state.pressed, k)
		}
		return m, studioTickCmd()
	}
	return m, nil
}

// restart re-reads path from disk and reloads it fresh -- editing the
// file in another window/terminal while `crust studio` is open and
// pressing `r` picks up the change, the same "just press the key
// again" workflow the Editor tab's own nvim reload loop has, without
// needing to leave this tool to get it.
func (m studioModel) restart() (tea.Model, tea.Cmd) {
	src, err := readStudioSource(m.path)
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	cols, rows := m.width, m.height
	if m.state != nil {
		cols, rows = m.state.cols, m.state.rows
	}
	interp, state, errMsg := loadStudioProgram(src, cols, rows)
	if errMsg != "" {
		m.err = errMsg
		return m, nil
	}
	m.interp = interp
	m.state = state
	m.err = ""
	return m, nil
}

func (m studioModel) View() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("🍕 crust studio") + "  " + styleMuted.Render(filepath.Base(m.path)))
	b.WriteByte('\n')

	if m.err != "" {
		b.WriteString(styleError.Render(m.err))
		b.WriteByte('\n')
	} else if m.state != nil {
		b.WriteString(m.renderStage())
		b.WriteByte('\n')
	}

	help := "q: quit   r: restart"
	if m.state != nil && m.state.lastLog != "" {
		help += "   deliver(): " + m.state.lastLog
	}
	b.WriteString(styleHelp.Render(help))
	return b.String()
}

// renderStage paints state.entities onto a blank cols x rows grid --
// rebuilt from scratch every tick rather than incrementally patched,
// the simplest correct thing at terminal scale (at most a few thousand
// cells) and the same "just redraw everything" posture every other
// tab in this TUI already takes.
func (m studioModel) renderStage() string {
	cols, rows := m.state.cols, m.state.rows
	grid := make([][]rune, rows)
	colorAt := make([][]string, rows)
	for y := range grid {
		grid[y] = make([]rune, cols)
		colorAt[y] = make([]string, cols)
		for x := range grid[y] {
			grid[y][x] = ' '
		}
	}
	for _, e := range m.state.entities {
		if e.x < 0 || e.x >= cols || e.y < 0 || e.y >= rows {
			continue
		}
		grid[e.y][e.x] = e.ch
		colorAt[e.y][e.x] = e.color
	}

	var b strings.Builder
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			ch := grid[y][x]
			if color := colorAt[y][x]; color != "" {
				b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(string(ch)))
			} else {
				b.WriteRune(ch)
			}
		}
		if y < rows-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
