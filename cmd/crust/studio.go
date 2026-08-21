// `crust studio <file.crust>` — a terminal-native counterpart to
// `crust game`: a live game runtime rendered directly in the terminal
// via bubbletea/lipgloss (the same stack `crust develop` already
// uses) instead of a browser tab. See studio_tui.go's own doc comment
// for the shape, and studio_builtins.go for cell/setPos/keyDown/
// onFrame/random — a different vocabulary from crust game's
// rect/circle (a terminal cell is the drawing primitive here, not a
// shape to fill) but the same handle-based spawn/move/destroy
// lifecycle.
package main

import (
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
)

// parseStudioArgs takes exactly one file argument -- unlike `crust
// game` (browser-editable, so a file is optional), there's no in-tool
// editor here, so a file is the only way to give it a program to run.
func parseStudioArgs(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("expected a file (crust studio <file.crust>)")
	}
	if len(args) > 1 {
		return "", fmt.Errorf("takes exactly one file argument (got %q and %q)", args[0], args[1])
	}
	return args[0], nil
}

func readStudioSource(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// runStudio loads path once (failing here, before the TUI ever opens,
// on a bad path or a program that doesn't even parse/run its own
// top-level setup -- the same "fail before handing over the screen"
// posture crust develop's own emptyDebugView already has), then hands
// off to a bubbletea Program for the actual real-time run. Needs a
// genuine terminal -- unlike crust develop, there's no plain/piped
// fallback mode that would mean anything for a real-time game.
func runStudio(path string, stdin io.Reader, stdout, stderr io.Writer) int {
	if !isColorTerminal(stdout) {
		fmt.Fprintln(stderr, "crust studio: needs a real terminal (output is piped or redirected)")
		return 1
	}

	src, err := readStudioSource(path)
	if err != nil {
		fmt.Fprintf(stderr, "crust studio: %s\n", err)
		return 1
	}

	cols, rows := 80, 22
	if f, ok := stdout.(*os.File); ok {
		if w, h, sizeErr := term.GetSize(f.Fd()); sizeErr == nil {
			cols, rows = studioStageDims(w, h)
		}
	}

	interp, state, errMsg := loadStudioProgram(src, cols, rows)
	if errMsg != "" {
		fmt.Fprintf(stderr, "crust studio: %s\n", errMsg)
		return 1
	}

	m := studioModel{path: path, interp: interp, state: state, width: cols, height: rows + 2}

	progOpts := []tea.ProgramOption{tea.WithOutput(stdout), tea.WithAltScreen()}
	if f, ok := stdin.(*os.File); ok {
		progOpts = append(progOpts, tea.WithInput(stdinNoNamer{f}))
	}
	prog := tea.NewProgram(m, progOpts...)
	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(stderr, "crust studio: %v\n", err)
		return 1
	}
	return 0
}
