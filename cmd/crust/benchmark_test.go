// Performance benchmarks through the real CLI pipeline (runFile),
// against a synthetic but puzzle-realistic-scale AoC 2020 Day 1 input
// -- catching interpreter performance regressions on real nested-loop
// AoC-shaped code before they'd show up mid-contest, not a
// microbenchmark of one isolated interpreter feature. Day 1 specifically:
// its two solutions are the closest thing among examples/aoc2020/ to a
// worst-case loop/index workload (part 1's O(n^2) pair search, part
// 2's O(n^3) triple search), so it's the one most likely to notice a
// regression in identifier lookup, index-expression evaluation, or
// loop overhead. The puzzle's own documented example (6 entries) is
// far too small for that -- real personal Day 1 inputs are, famously,
// exactly 200 lines, which is the scale aoc2020Day1BenchmarkInput
// reproduces.
package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// aoc2020Day1BenchmarkInput generates a deterministic (fixed seed, so
// every benchmark run does identical work) synthetic expense report:
// 200 distinct entries in [1, 2019], the same size real AoC 2020 Day 1
// personal inputs use. A known pair (673, 1347) and an additional
// entry completing a known triple (673, 1000, 347) are appended last
// rather than left to chance, specifically so day01.crust's
// early-exits-on-first-match nested loops do close to their full
// search before finding an answer -- the representative case, not the
// lucky-early-match one a purely random list might produce.
func aoc2020Day1BenchmarkInput() string {
	rng := rand.New(rand.NewSource(20201201))
	seen := map[int]bool{673: true, 1347: true, 1000: true, 347: true}
	nums := make([]int, 0, 200)
	for len(nums) < 196 {
		n := rng.Intn(2019) + 1
		if seen[n] {
			continue
		}
		seen[n] = true
		nums = append(nums, n)
	}
	nums = append(nums, 673, 1347, 1000, 347)

	var b strings.Builder
	for _, n := range nums {
		fmt.Fprintln(&b, n)
	}
	return b.String()
}

func BenchmarkAoC2020Day1Part1(b *testing.B) {
	input := aoc2020Day1BenchmarkInput()
	const path = "../../examples/aoc2020/day01.crust"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var stdout, stderr bytes.Buffer
		if code := runFile(path, "part1", strings.NewReader(input), &stdout, &stderr); code != 0 {
			b.Fatalf("part1 failed: %s", stderr.String())
		}
	}
}

func BenchmarkAoC2020Day1Part2(b *testing.B) {
	input := aoc2020Day1BenchmarkInput()
	const path = "../../examples/aoc2020/day01.crust"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var stdout, stderr bytes.Buffer
		if code := runFile(path, "part2", strings.NewReader(input), &stdout, &stderr); code != 0 {
			b.Fatalf("part2 failed: %s", stderr.String())
		}
	}
}
