// TestRealExamplesProduceExpectedOutput runs real shipped .crust
// programs -- not synthetic snippets -- through the actual CLI
// pipeline (runFile) and pins their output to values verified
// independently of this test suite (the AoC ones against the puzzle's
// own documented example answers, verified while writing
// examples/aoc2020/ itself; see TODO.md's Phase 8 dry-run entry).
// TestRunFile (main_test.go) already covers runFile's own dispatch/
// error-handling logic with small synthetic snippets built inline;
// this file is deliberately the opposite -- real programs, real (if
// puzzle-example-scale) input, exact output -- so a change that
// quietly breaks what the shipped examples actually compute gets
// caught here rather than by a puzzled user.
package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestRealExamplesProduceExpectedOutput(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		store     string
		inputPath string // "" = no stdin
		want      string
	}{
		{"aoc2020 day01 part1", "../../examples/aoc2020/day01.crust", "part1", "../../examples/aoc2020/day01_input.txt", "514579"},
		{"aoc2020 day01 part2", "../../examples/aoc2020/day01.crust", "part2", "../../examples/aoc2020/day01_input.txt", "241861950"},
		{"aoc2020 day02 part1", "../../examples/aoc2020/day02.crust", "part1", "../../examples/aoc2020/day02_input.txt", "2"},
		{"aoc2020 day02 part2", "../../examples/aoc2020/day02.crust", "part2", "../../examples/aoc2020/day02_input.txt", "1"},
		{"aoc2020 day03 part1", "../../examples/aoc2020/day03.crust", "part1", "../../examples/aoc2020/day03_input.txt", "7"},
		{"aoc2020 day03 part2", "../../examples/aoc2020/day03.crust", "part2", "../../examples/aoc2020/day03_input.txt", "336"},
		{"aoc2020 day04 part1", "../../examples/aoc2020/day04.crust", "part1", "../../examples/aoc2020/day04_input.txt", "2"},
		{"aoc2020 day04 part2", "../../examples/aoc2020/day04.crust", "part2", "../../examples/aoc2020/day04_part2_input.txt", "2"},
		{"aoc2020 day05 part1", "../../examples/aoc2020/day05.crust", "part1", "../../examples/aoc2020/day05_input.txt", "820"},
		{"aoc2020 day05 part2", "../../examples/aoc2020/day05.crust", "part2", "../../examples/aoc2020/day05_input.txt", "no missing seat in this example set (needs full-scale puzzle input for a real gap)"},
		{"day01_find_pair (shape-only example)", "../../examples/day01_find_pair.crust", "", "", "514579"},
		{"day06_group_answers (shape-only example)", "../../examples/day06_group_answers.crust", "", "", "3\n4"},
		{"module_demo (delivery / module system)", "../../examples/module_demo.crust", "", "", "7\n15"},
		{"day1_essentials (Phase 8 confirmation pass)", "../../examples/day1_essentials.crust", "", "../../examples/day1_essentials_input.txt",
			"hello from a real file\n12\n1\n2\n3.5\ncRust 2026\na-b-c\nPEPPERONI basil\nstuffed\n12\n6\n3\n" +
				"[1, 2, 3, 4, 5, 6]\n[2, 3, 4]\n[1, 4, 9, 16, 25, 36]\n[2, 4, 6]\n21\n10\nnobox\n[alice, bob, carol]\n20\n[alice, carol]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdin strings.Reader
			if tt.inputPath != "" {
				data, err := os.ReadFile(tt.inputPath)
				if err != nil {
					t.Fatalf("reading input file: %s", err)
				}
				stdin = *strings.NewReader(string(data))
			}

			var stdout, stderr bytes.Buffer
			code := runFile(tt.path, tt.store, &stdin, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
			}
			if got := strings.TrimRight(stdout.String(), "\n"); got != tt.want {
				t.Errorf("stdout = %q, want %q", got, tt.want)
			}
		})
	}
}
