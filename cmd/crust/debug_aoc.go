// AoC input auto-fetch and the live stopwatch shown in `crust
// develop`'s TUI header — both fully optional, silently inert unless
// a session cookie has been saved (`crust login`). Neither this file
// nor internal/aoc ever makes develop itself require network access:
// every entry point here degrades to a no-op the moment LoadSession
// reports nothing saved.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Sintfoap/cRust/internal/aoc"
)

// dayNumberFromPath extracts the day number from a filename following
// the examples/dayNN_template.crust convention ("day06.crust",
// "day6_input.txt", "Day06_part2.crust", ...): "day" (any case) then
// one or more digits, at the very start of the base filename. Not
// every .crust file follows this convention (day1_essentials.crust
// does, but plenty of ad-hoc scratch files won't parse) — ok is false
// for anything that doesn't match, and every caller here treats that
// as "the AoC features just don't apply to this file," never an
// error.
func dayNumberFromPath(path string) (day int, ok bool) {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	lower := strings.ToLower(base)
	if !strings.HasPrefix(lower, "day") {
		return 0, false
	}
	rest := base[len("day"):]
	i := 0
	for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, false
	}
	n, err := strconv.Atoi(rest[:i])
	if err != nil || n < 1 || n > 25 {
		return 0, false
	}
	return n, true
}

// aocInputPath is where day's input belongs alongside path, per
// examples/dayNN_template.crust's own naming convention — always
// zero-padded to two digits regardless of how path itself was
// written, since that's the convention the template documents.
func aocInputPath(path string, day int) string {
	return filepath.Join(filepath.Dir(path), fmt.Sprintf("day%02d_input.txt", day))
}

// maybeAutoFetchInput downloads path's day's puzzle input and starts
// its timer, but only when every one of these hold: path's filename
// parses as a day number, a session cookie has been saved, and the
// input file isn't already sitting there (never overwrites something
// that already exists — same protection `crust fetch` gives without
// --force). Any fetch failure (day not unlocked yet, bad cookie, a
// network error) is reported but never fatal to `crust develop`
// itself — the whole feature is a convenience on top of a workflow
// that works fine without it.
func maybeAutoFetchInput(path string, stderr io.Writer) {
	day, ok := dayNumberFromPath(path)
	if !ok {
		return
	}
	session, ok := aoc.LoadSession()
	if !ok {
		return
	}
	inputPath := aocInputPath(path, day)
	if _, err := os.Stat(inputPath); err == nil {
		return
	}

	client := &aoc.Client{Session: session, BaseURL: fetchBaseURL}
	data, err := client.FetchInput(defaultAoCYear, day)
	if err != nil {
		fmt.Fprintf(stderr, "crust develop: auto-fetch for day %d failed: %s\n", day, err)
		return
	}
	if err := os.WriteFile(inputPath, data, 0o644); err != nil {
		fmt.Fprintf(stderr, "crust develop: auto-fetch: writing %s: %s\n", inputPath, err)
		return
	}
	_ = aoc.StartTimer(defaultAoCYear, day)
	fmt.Fprintf(stderr, "crust develop: fetched day %d input to %s — timer started\n", day, inputPath)
}

// aocTimerRunning reports whether m's current file has a day number
// and that day's timer is currently running — the condition under
// which the header's stopwatch needs to keep ticking at all.
func (m debugModel) aocTimerRunning() bool {
	day, ok := dayNumberFromPath(m.view.path)
	if !ok {
		return false
	}
	_, running := aoc.Elapsed(defaultAoCYear, day)
	return running
}

// aocStatusLine renders the header's trailing stopwatch text, or ""
// if this file has no day number or no timer record at all (a file
// nobody's ever `crust fetch`ed or `crust done`d shows nothing —
// there's no "0s, not started" clutter for solvers not using the
// feature).
func (m debugModel) aocStatusLine() string {
	day, ok := dayNumberFromPath(m.view.path)
	if !ok {
		return ""
	}
	elapsed, running := aoc.Elapsed(defaultAoCYear, day)
	if elapsed == 0 && !running {
		return ""
	}
	state := "stopped"
	if running {
		state = "running"
	}
	return fmt.Sprintf("  ⏱ day %d: %s (%s)", day, elapsed.Round(time.Second), state)
}

// aocTickMsg drives the header's live stopwatch: a redraw once a
// second for as long as aocTimerRunning holds, so the elapsed time
// keeps advancing on screen even with no keypresses at all. Ticking
// stops itself the moment the timer isn't running anymore (stopped by
// `crust done`, possibly in another terminal) instead of running
// forever in the background.
type aocTickMsg time.Time

func aocTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return aocTickMsg(t) })
}
