// `crust fetch <day>` — download that day's puzzle input from
// adventofcode.com using the session cookie `crust login` saved (or
// $AOC_SESSION), and write it to disk as dayNN_input.txt — the same
// filename convention examples/dayNN_template.crust's own header
// comment already tells a solver to use.
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/Sintfoap/cRust/internal/aoc"
)

// defaultAoCYear is the event this whole project exists for; --year
// only needs to be passed to fetch a different one.
const defaultAoCYear = 2026

// fetchBaseURL overrides aoc.Client's BaseURL when non-empty; tests
// point it at an httptest.Server instead of the real
// adventofcode.com, the same swap-a-package-var-for-tests pattern
// debug_state.go's develStateDir already uses.
var fetchBaseURL string

// parseFetchArgs pulls a day number and the optional --year/--force/
// --out flags out of args, in any order — the same flexibility
// parseRunArgs/parseFmtArgs already give their own commands.
func parseFetchArgs(args []string) (day, year int, force bool, out string, err error) {
	year = defaultAoCYear
	dayStr := ""

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
		case a == "--force":
			force = true
		case a == "--year" || strings.HasPrefix(a, "--year="):
			v, e := takeVal("--year")
			if e != nil {
				return 0, 0, false, "", e
			}
			n, e := strconv.Atoi(v)
			if e != nil {
				return 0, 0, false, "", fmt.Errorf("invalid --year %q", v)
			}
			year = n
		case a == "--out" || strings.HasPrefix(a, "--out="):
			v, e := takeVal("--out")
			if e != nil {
				return 0, 0, false, "", e
			}
			out = v
		case strings.HasPrefix(a, "-"):
			return 0, 0, false, "", fmt.Errorf("unknown flag: %s", a)
		default:
			if dayStr != "" {
				return 0, 0, false, "", fmt.Errorf("unexpected extra argument: %s", a)
			}
			dayStr = a
		}
	}

	if dayStr == "" {
		return 0, 0, false, "", fmt.Errorf("missing <day>")
	}
	day, convErr := strconv.Atoi(dayStr)
	if convErr != nil || day < 1 || day > 25 {
		return 0, 0, false, "", fmt.Errorf("invalid day %q (want 1-25)", dayStr)
	}

	if out == "" {
		out = fmt.Sprintf("day%02d_input.txt", day)
	}
	return day, year, force, out, nil
}

// runFetch downloads day/year's puzzle input and writes it to out,
// refusing to clobber an existing file unless force is set — the same
// protection against accidentally discarding a manually-edited input
// file that a plain `> file.txt` redirect wouldn't give.
func runFetch(day, year int, force bool, out string, stdout, stderr io.Writer) int {
	session, ok := aoc.LoadSession()
	if !ok {
		fmt.Fprintln(stderr, "crust fetch: no session cookie saved — run `crust login` first (or set $AOC_SESSION)")
		return 1
	}

	if !force {
		if _, err := os.Stat(out); err == nil {
			fmt.Fprintf(stderr, "crust fetch: %s already exists (use --force to overwrite)\n", out)
			return 1
		}
	}

	client := &aoc.Client{Session: session, BaseURL: fetchBaseURL}
	data, err := client.FetchInput(year, day)
	if err != nil {
		fmt.Fprintf(stderr, "crust fetch: %s\n", err)
		return 1
	}

	if err := os.WriteFile(out, data, 0o644); err != nil {
		fmt.Fprintf(stderr, "crust fetch: writing %s: %s\n", out, err)
		return 1
	}

	_ = aoc.StartTimer(year, day)
	fmt.Fprintf(stdout, "Saved day %d, %d input to %s (%d bytes). Timer started — `crust done %d` when you've got the star.\n", day, year, out, len(data), day)
	return 0
}
