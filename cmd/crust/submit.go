// `crust submit <day> [answer]` — submit that day's answer to
// adventofcode.com using the session cookie `crust login` saved (or
// $AOC_SESSION), the same auth `crust fetch` already relies on.
// Deliberately doesn't touch the timer `crust fetch` starts and `crust
// done` stops — a correct part 1 isn't "done" for the day, and only
// the solver knows when they're actually finished, the same reasoning
// done.go's own doc comment already gives for staying hands-off there.
package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Sintfoap/cRust/internal/aoc"
)

// submitBaseURL overrides aoc.Client's BaseURL when non-empty, the
// same test-only swap fetchBaseURL already uses.
var submitBaseURL string

// parseSubmitArgs pulls a day number, optional answer, required
// --part=<1|2>, and optional --year out of args, in any order — the
// same flexibility parseFetchArgs already gives. answer comes back
// empty when omitted from args, which runSubmit takes as "read it from
// stdin instead" — the same shape unbox() already gives cRust
// programs, so `crust run day03.crust --store=part1 | crust submit 3
// --part=1` pipes straight through without a manual copy-paste step.
func parseSubmitArgs(args []string) (day, part, year int, answer string, err error) {
	year = defaultAoCYear
	dayStr := ""
	haveAnswer := false

	for i := 0; i < len(args); i++ {
		a := args[i]
		takeVal := func(flag string) (string, error) {
			if v, ok := strings.CutPrefix(a, flag+"="); ok {
				return v, nil
			}
			i++
			if i >= len(args) {
				return "", fmt.Errorf("%s requires a value", flag)
			}
			return args[i], nil
		}

		switch {
		case a == "--part" || strings.HasPrefix(a, "--part="):
			v, e := takeVal("--part")
			if e != nil {
				return 0, 0, 0, "", e
			}
			n, e := strconv.Atoi(v)
			if e != nil || (n != 1 && n != 2) {
				return 0, 0, 0, "", fmt.Errorf("invalid --part %q (want 1 or 2)", v)
			}
			part = n
		case a == "--year" || strings.HasPrefix(a, "--year="):
			v, e := takeVal("--year")
			if e != nil {
				return 0, 0, 0, "", e
			}
			n, e := strconv.Atoi(v)
			if e != nil {
				return 0, 0, 0, "", fmt.Errorf("invalid --year %q", v)
			}
			year = n
		case strings.HasPrefix(a, "-"):
			return 0, 0, 0, "", fmt.Errorf("unknown flag: %s", a)
		default:
			switch {
			case dayStr == "":
				dayStr = a
			case !haveAnswer:
				answer = a
				haveAnswer = true
			default:
				return 0, 0, 0, "", fmt.Errorf("unexpected extra argument: %s", a)
			}
		}
	}

	if dayStr == "" {
		return 0, 0, 0, "", fmt.Errorf("missing <day>")
	}
	day, convErr := strconv.Atoi(dayStr)
	if convErr != nil || day < 1 || day > 25 {
		return 0, 0, 0, "", fmt.Errorf("invalid day %q (want 1-25)", dayStr)
	}
	if part == 0 {
		return 0, 0, 0, "", fmt.Errorf("missing --part=1 or --part=2")
	}
	return day, part, year, answer, nil
}

// runSubmit submits answer (or, if empty, the first line read from
// stdin) as day/year's part-th answer, printing AoC's own verdict and
// mapping it to an exit code a script can branch on: 0 for a correct
// answer, 1 for anything else (wrong, rate-limited, already solved,
// unrecognized, or a request that failed outright) — the same
// success-is-0 convention every other crust subcommand already uses,
// so `crust submit ... && crust done N` chains naturally.
func runSubmit(day, part, year int, answer string, stdin io.Reader, stdout, stderr io.Writer) int {
	if answer == "" {
		scanner := bufio.NewScanner(stdin)
		if !scanner.Scan() {
			fmt.Fprintln(stderr, "crust submit: no answer given (pass one as an argument or pipe it on stdin)")
			return 1
		}
		answer = strings.TrimSpace(scanner.Text())
		if answer == "" {
			fmt.Fprintln(stderr, "crust submit: empty answer")
			return 1
		}
	}

	session, ok := aoc.LoadSession()
	if !ok {
		fmt.Fprintln(stderr, "crust submit: no session cookie saved — run `crust login` first (or set $AOC_SESSION)")
		return 1
	}

	client := &aoc.Client{Session: session, BaseURL: submitBaseURL}
	result, err := client.Submit(year, day, part, answer)
	if err != nil {
		fmt.Fprintf(stderr, "crust submit: %s\n", err)
		return 1
	}

	switch result.Outcome {
	case aoc.OutcomeCorrect:
		fmt.Fprintf(stdout, "Correct! Day %d part %d, %d: %s\n", day, part, year, result.Message)
		return 0
	case aoc.OutcomeTooLow:
		fmt.Fprintf(stdout, "Wrong (too low). %s\n", result.Message)
	case aoc.OutcomeTooHigh:
		fmt.Fprintf(stdout, "Wrong (too high). %s\n", result.Message)
	case aoc.OutcomeIncorrect:
		fmt.Fprintf(stdout, "Wrong. %s\n", result.Message)
	case aoc.OutcomeRateLimited:
		fmt.Fprintf(stdout, "Rate limited: %s\n", result.Message)
	case aoc.OutcomeAlreadySolved:
		fmt.Fprintf(stdout, "%s\n", result.Message)
	default:
		fmt.Fprintf(stdout, "%s\n", result.Message)
	}
	return 1
}
