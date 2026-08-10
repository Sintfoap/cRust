// `crust new <day>` stamps out a fresh dayNN.crust from the same
// store_part1/store_part2 starter shape examples/dayNN_template.crust
// already documents for manual copying — this command does the "cp
// and rename" step itself, with the real day number filled in where
// the template file's own copy still says "NN" as a placeholder for a
// human to replace by hand. The new file lands in the current
// directory using the same day%02d.crust naming `crust fetch`/`crust
// done` already expect, so it's immediately visible in `crust
// develop`'s own Nav tab (any .crust file "alongside" the one
// currently open) without either side needing to know about the
// other.
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// dayFileTemplateFmt is filled in with the year once and the day
// number six times (the file header, the `crust develop` line, and
// each of the two `crust run --store=` example lines needing it
// twice) via fmt.Sprintf. Deliberately the same store_part1/
// store_part2 shape examples/dayNN_template.crust uses — one shape
// taught in one place — rather than inventing a second starter layout
// just because this one gets its placeholders filled in automatically
// instead of by hand.
const dayFileTemplateFmt = `// AoC %d Day %02d starter.
//
//   crust develop day%02d.crust           // step through it as you write it
//   crust run day%02d.crust --store=part1 < day%02d_input.txt
//   crust run day%02d.crust --store=part2 < day%02d_input.txt
//
// unbox() reads whichever input crust run/crust develop was given
// (stdin, or the Run tab's input-file field) -- lines(unbox()) splits
// it into a List of lines, ints(...) turns numeric strings into
// Integers. See docs/CHEATSHEET.md for the full builtin list and a
// few common parsing idioms (grid puzzles, "key: value" lines,
// memoization, ...).

recipe store_part1() {
    input = lines(unbox())
    deliver(slices(input))
}

recipe store_part2() {
    input = lines(unbox())
    deliver(slices(input))
}
`

// parseNewArgs pulls a day number and the optional --year/--force
// flags out of args, in any order — the same shape parseFetchArgs/
// parseDoneArgs already give their own commands, minus --out since a
// new day's file always follows the one fixed dayNN.crust naming.
// year comes back 0 when --year is omitted, the same "0 means not
// passed" convention debugOptions.Year already uses — runNew treats
// that as "stamp the template with defaultAoCYear, but don't persist
// anything," so an ordinary `crust new 7` (the common case, this
// year's puzzle) never writes a Year entry into develop_state.json for
// a file that never actually needed one.
func parseNewArgs(args []string) (day, year int, force bool, err error) {
	dayStr := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--force":
			force = true
		case a == "--year" || strings.HasPrefix(a, "--year="):
			v, ok := strings.CutPrefix(a, "--year=")
			if !ok {
				i++
				if i >= len(args) {
					return 0, 0, false, fmt.Errorf("--year requires a value")
				}
				v = args[i]
			}
			n, convErr := strconv.Atoi(v)
			if convErr != nil {
				return 0, 0, false, fmt.Errorf("invalid --year %q", v)
			}
			year = n
		case len(a) > 0 && a[0] == '-':
			return 0, 0, false, fmt.Errorf("unknown flag: %s", a)
		default:
			if dayStr != "" {
				return 0, 0, false, fmt.Errorf("unexpected extra argument: %s", a)
			}
			dayStr = a
		}
	}
	if dayStr == "" {
		return 0, 0, false, fmt.Errorf("missing <day>")
	}
	day, convErr := strconv.Atoi(dayStr)
	if convErr != nil || day < 1 || day > 25 {
		return 0, 0, false, fmt.Errorf("invalid day %q (want 1-25)", dayStr)
	}
	return day, year, force, nil
}

// runNew writes dayNN.crust from dayFileTemplateFmt, refusing to
// clobber an existing file unless force is set — the same protection
// runFetch already gives an existing input file, for the same reason:
// a file a solver has already started writing in is exactly the thing
// this guard exists to not silently discard.
//
// An explicit year (0 means none was passed) is both stamped into the
// file's own header comment and saved immediately as this file's
// remembered AoC event (debug_state.go's saveYearBestEffort) — the
// same "sticks the moment it's given" reasoning debugOptions.Year's
// own doc comment gives `crust develop --year`, so setting up a past
// year is genuinely one step: `crust new 7 --year 2020` leaves
// `crust develop day07.crust`/its auto-fetch already knowing 2020,
// with no second --year to remember on every later invocation.
func runNew(day, year int, force bool, stdout, stderr io.Writer) int {
	path := fmt.Sprintf("day%02d.crust", day)
	if !force {
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(stderr, "crust new: %s already exists (use --force to overwrite)\n", path)
			return 1
		}
	}

	displayYear := year
	if displayYear == 0 {
		displayYear = defaultAoCYear
	}
	content := fmt.Sprintf(dayFileTemplateFmt, displayYear, day, day, day, day, day, day)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		fmt.Fprintf(stderr, "crust new: writing %s: %s\n", path, err)
		return 1
	}
	if year != 0 {
		saveYearBestEffort(path, year)
	}

	fmt.Fprintf(stdout, "Created %s (AoC %d). `crust fetch %d --year %d` to grab its input, `crust develop %s` to start writing.\n", path, displayYear, day, displayYear, path)
	return 0
}
