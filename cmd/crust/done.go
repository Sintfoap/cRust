// `crust done <day>` — stop that day's timer (started by `crust
// fetch`, or `crust develop`'s auto-fetch) and print how long it
// took. Deliberately not automatic on a clean run: nothing here
// checks a run's output against AoC's own accepted answer, so a
// program finishing without error doesn't mean the day is actually
// solved — the solver is the only one who knows that.
package main

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/Sintfoap/cRust/internal/aoc"
)

// parseDoneArgs pulls a day number and the optional --year flag out of
// args, matching parseFetchArgs' own flag shape.
func parseDoneArgs(args []string) (day, year int, err error) {
	year = defaultAoCYear
	dayStr := ""

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--year" || strings.HasPrefix(a, "--year="):
			v, ok := strings.CutPrefix(a, "--year=")
			if !ok {
				i++
				if i >= len(args) {
					return 0, 0, fmt.Errorf("--year requires a value")
				}
				v = args[i]
			}
			n, e := strconv.Atoi(v)
			if e != nil {
				return 0, 0, fmt.Errorf("invalid --year %q", v)
			}
			year = n
		case strings.HasPrefix(a, "-"):
			return 0, 0, fmt.Errorf("unknown flag: %s", a)
		default:
			if dayStr != "" {
				return 0, 0, fmt.Errorf("unexpected extra argument: %s", a)
			}
			dayStr = a
		}
	}

	if dayStr == "" {
		return 0, 0, fmt.Errorf("missing <day>")
	}
	day, convErr := strconv.Atoi(dayStr)
	if convErr != nil || day < 1 || day > 25 {
		return 0, 0, fmt.Errorf("invalid day %q (want 1-25)", dayStr)
	}
	return day, year, nil
}

// runDone stops day/year's timer and reports the total elapsed time.
func runDone(day, year int, stdout, stderr io.Writer) int {
	if err := aoc.StopTimer(year, day); err != nil {
		fmt.Fprintf(stderr, "crust done: %s\n", err)
		return 1
	}

	elapsed, _ := aoc.Elapsed(year, day)
	fmt.Fprintf(stdout, "Day %d, %d: %s\n", day, year, elapsed.Round(time.Second))
	return 0
}
