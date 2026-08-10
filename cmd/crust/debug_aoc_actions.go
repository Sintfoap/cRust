// Manual AoC actions inside `crust develop` itself — fetch, submit,
// and login — on direct request: "add manual options for the crust
// login/fetch/submit/new in the develop tool in places that make
// sense." Auto-fetch (debug_aoc.go) already covers the common case
// (open a file, its input shows up), but a solver still needs a way to
// retry a failed auto-fetch, submit an answer without leaving the TUI,
// or log in for the first time from inside an already-open session —
// none of which the passive auto-fetch path can do on its own.
//
// All three live on the Run tab, bound to ctrl+f/ctrl+s/ctrl+l — never
// bare letters, since the Run tab's input-file field needs every
// printable key for itself (handleRunTabKey's own doc comment already
// explains why Ctrl+R/PgUp/PgDn are the only letter-adjacent keys
// reserved there). Ctrl+<letter> combos are safe for the same reason
// Ctrl+R already is: a terminal in raw mode never generates one from
// ordinary typing, so binding one can never eat a character a real
// file path might contain.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Sintfoap/cRust/internal/aoc"
)

// aocFetchMsg carries ctrl+f's result: either the path it landed at
// plus a human-readable status line, or an error. inputPath is "" on
// failure, since handleAocFetchResult only touches m.runInput when
// there's an actual path to point it at.
type aocFetchMsg struct {
	inputPath string
	status    string
	err       string
}

// aocFetchCmd downloads (or, if already present, simply adopts) this
// file's day/year input, the same as maybeAutoFetchInput's own startup
// path but reachable mid-session and without the "only if missing"
// silent-skip — pressing ctrl+f is an explicit request, so an input
// file that's already there is reported as a (successful) no-op rather
// than nothing happening with no feedback at all. day/year/store come
// from the model up front, not read again inside the returned Cmd,
// the same snapshot-before-the-closure shape runProgramCmd already
// uses.
func (m debugModel) aocFetchCmd() tea.Cmd {
	path := m.view.path
	year := m.opts.Year

	return func() tea.Msg {
		day, ok := dayNumberFromPath(path)
		if !ok {
			return aocFetchMsg{err: "this file's name doesn't parse as a dayNN.crust puzzle"}
		}
		inputPath := aocInputPath(path, day)

		if _, err := os.Stat(inputPath); err == nil {
			saveInputPathBestEffort(path, inputPath)
			return aocFetchMsg{
				inputPath: inputPath,
				status:    fmt.Sprintf("day %d, %d input already present — wired into the input field", day, year),
			}
		}

		session, ok := aoc.LoadSession()
		if !ok {
			return aocFetchMsg{err: "no session cookie saved — ctrl+l to log in first"}
		}

		client := &aoc.Client{Session: session, BaseURL: fetchBaseURL}
		data, err := client.FetchInput(year, day)
		if err != nil {
			return aocFetchMsg{err: err.Error()}
		}
		if err := os.WriteFile(inputPath, data, 0o644); err != nil {
			return aocFetchMsg{err: err.Error()}
		}
		_ = aoc.StartTimer(year, day)
		saveInputPathBestEffort(path, inputPath)
		return aocFetchMsg{
			inputPath: inputPath,
			status:    fmt.Sprintf("fetched day %d, %d input (%d bytes) — timer started", day, year, len(data)),
		}
	}
}

// handleAocFetchResult applies ctrl+f's outcome: an error just becomes
// the Run tab's status line, while a success also replaces m.runInput
// with the fetched path outright — ctrl+f is an explicit "get this
// day's input" request, so overwriting whatever was typed there
// before is the whole point, the same "an explicit action wins"
// posture the CLI's own `crust fetch` already takes toward an existing
// file (refuses to clobber it, but this path only reaches here once
// that's not a concern). Restarting the tick loop mirrors every other
// place a timer might have just started (handleKey's own tabNav enter
// case, createNavFile) — aocTickCmd is idempotent to call again if one
// was already scheduled, so there's no harm doing it unconditionally
// when the timer turns out to be running.
func (m debugModel) handleAocFetchResult(msg aocFetchMsg) (tea.Model, tea.Cmd) {
	if msg.err != "" {
		m.aocActionStatus = "fetch failed: " + msg.err
		return m, nil
	}
	m.aocActionStatus = msg.status
	value := []rune(msg.inputPath)
	m.runInput = runInputModel{value: value, cursor: len(value)}
	if m.aocTimerRunning() {
		return m, aocTickCmd()
	}
	return m, nil
}

// levelForEntry maps a Run tab entry-point --store value to AoC's own
// "level" concept (1 or 2, i.e. part1/part2) for ctrl+s — "part2"
// means level 2, everything else (the bare store, "part1", or any
// other name a file happens to use) means level 1. Good enough for
// every file this session's own tooling ever generates (crust new's
// template always uses exactly store_part1/store_part2), and a level-1
// default is the safer guess for anything else, since most single-part
// AoC days only have a level 1 to submit against anyway.
func levelForEntry(store string) int {
	if store == "part2" {
		return 2
	}
	return 1
}

// lastNonEmptyLine returns the last non-blank, trimmed line of s, or
// "" if there isn't one — ctrl+s's answer source: the Run tab's output
// panel already shows a run's own deliver() output verbatim, and the
// final line is what a solver would read off the screen and paste into
// AoC's own website by hand, so it's the most direct stand-in for
// "the answer" without asking for a separate typed-in value.
func lastNonEmptyLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line
		}
	}
	return ""
}

// aocSubmitMsg carries ctrl+s's result: a status line describing what
// was submitted and AoC's verdict, or an error.
type aocSubmitMsg struct {
	status string
	err    string
}

// aocSubmitCmd submits the last run's own final output line as this
// file's day/year/level answer — level from the Run tab's currently
// selected entry point (levelForEntry), day/year the same way
// aocFetchCmd resolves them. Refuses before ever making a request when
// there's nothing sensible to submit: no run yet, or the last run
// failed (its final line is far more likely to be error text than an
// actual answer) — the same "don't do something surprising with bad
// input" posture runSubmit's own empty-answer guard already takes at
// the CLI layer.
func (m debugModel) aocSubmitCmd() tea.Cmd {
	path := m.view.path
	year := m.opts.Year
	store := m.selectedRunEntry()
	output := m.runOutput
	failed := m.runFailed

	return func() tea.Msg {
		day, ok := dayNumberFromPath(path)
		if !ok {
			return aocSubmitMsg{err: "this file's name doesn't parse as a dayNN.crust puzzle"}
		}
		if output == "" {
			return aocSubmitMsg{err: "nothing to submit yet — enter runs the program first"}
		}
		if failed {
			return aocSubmitMsg{err: "last run failed — fix it and run again before submitting"}
		}
		answer := lastNonEmptyLine(output)
		if answer == "" {
			return aocSubmitMsg{err: "nothing to submit yet — enter runs the program first"}
		}

		session, ok := aoc.LoadSession()
		if !ok {
			return aocSubmitMsg{err: "no session cookie saved — ctrl+l to log in first"}
		}

		level := levelForEntry(store)
		client := &aoc.Client{Session: session, BaseURL: submitBaseURL}
		result, err := client.Submit(year, day, level, answer)
		if err != nil {
			return aocSubmitMsg{err: err.Error()}
		}

		verb := "Wrong."
		switch result.Outcome {
		case aoc.OutcomeCorrect:
			verb = "Correct!"
		case aoc.OutcomeTooLow:
			verb = "Wrong (too low)."
		case aoc.OutcomeTooHigh:
			verb = "Wrong (too high)."
		case aoc.OutcomeRateLimited:
			verb = "Rate limited."
		case aoc.OutcomeAlreadySolved:
			verb = ""
		}
		status := strings.TrimSpace(fmt.Sprintf("submitted %q (day %d part %d, %d): %s %s", answer, day, level, year, verb, result.Message))
		return aocSubmitMsg{status: status}
	}
}

// handleAocSubmitResult applies ctrl+s's outcome to the Run tab's
// status line — success or failure alike, since a wrong answer is a
// perfectly normal outcome to show, not something to treat as this
// action having failed.
func (m debugModel) handleAocSubmitResult(msg aocSubmitMsg) (tea.Model, tea.Cmd) {
	if msg.err != "" {
		m.aocActionStatus = "submit failed: " + msg.err
		return m, nil
	}
	m.aocActionStatus = msg.status
	return m, nil
}

// loginExitMsg reports how ctrl+l's suspended login prompt ended.
type loginExitMsg struct {
	err error
}

// loginCmd suspends the develop TUI and hands the real terminal to a
// fresh `crust login` — self-exec'd (os.Args[0], not a hardcoded
// "crust") so this works identically for a `go run` build, a locally
// built binary under any name, or an installed one — reusing the
// ordinary interactive prompt (login.go's runLogin) rather than a
// second, TUI-flavored login flow. tea.ExecProcess is exactly the
// mechanism the Editor tab's own nvim hand-off already uses
// (openEditorCmd) for the identical reason: an interactive program
// that reads/writes the real terminal directly can't share the screen
// with bubbletea's own raw-mode rendering. cmd.Stdin is preset to the
// real os.Stdin for the same reason nvimCmd's own doc comment gives —
// bubbletea's ExecProcess only fills in Stdin when it's unset, and
// would otherwise hand the child a non-*os.File wrapper os/exec can't
// pass through as a real fd.
func loginCmd() tea.Cmd {
	cmd := exec.Command(os.Args[0], "login")
	cmd.Stdin = os.Stdin
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return loginExitMsg{err: err}
	})
}

// handleLoginExit reacts to the login prompt handing the terminal
// back. Deliberately checks aoc.LoadSession() itself rather than
// trusting msg.err: cmd.Run() reports a non-zero exit status as an
// error (e.g. the user hit Enter on an empty prompt, exactly the case
// runLogin itself already exits 1 for), which isn't the same question
// as "is there now a usable session" — checking the real, current
// state directly is simpler than trying to reverse-engineer that
// distinction from an exit code, and it's the one thing every other
// AoC action here (fetch, submit) actually cares about.
func (m debugModel) handleLoginExit(_ loginExitMsg) (tea.Model, tea.Cmd) {
	if _, ok := aoc.LoadSession(); ok {
		m.aocActionStatus = "session saved"
		return m, nil
	}
	m.aocActionStatus = "login exited without saving a session"
	return m, nil
}
