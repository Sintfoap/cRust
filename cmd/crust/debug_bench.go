// The Bench tab (debug_tui.go's tabBench): run the file being
// debugged N times, back to back, with whichever entry point and
// input file the Run tab currently has selected, and chart the
// resulting wall-clock time and memory-allocated numbers across those
// runs — on direct request: "write a KPI for the develop tool that
// does x number of runs of the current settings of the run tab and
// shows a graph on the average runtime, the average memory, the max
// runtime and memory, the min memory and runtime, and any other stats
// that would be notable... make it so any of the lines on the graph
// can be enabled or disable[d] depending on what I'd like to compare."
//
// Two separate mini charts, Runtime above Memory, each on its own
// scale — the same reasoning that split the original KPI tab into
// Time and Memory in the first place (debug_tui.go's own doc comment,
// per direct user feedback): overlaying nanoseconds and bytes on one
// shared axis would make neither legible. Both charts share one
// toggle set (raw per-run values, average, median, max, min), applied
// identically to each — "average" is one abstract series that shows
// up on both charts with different underlying numbers, not two
// independent concepts to track separately.
//
// Deliberately untraced: each of the N runs goes straight through
// runFile (run.go), the same function `crust run` itself and the Run
// tab's own "run" action use for their command-line-style output —
// not buildDebugView/debugger.Recorder, whose per-statement tracing
// overhead would badly distort exactly the timing numbers this
// feature exists to measure. cmd/crust/benchmark_test.go's own
// BenchmarkAoC2020Day1Part1 already established this same
// runFile-directly, no-tracer pattern for measuring real interpreter
// performance, for the identical reason.
package main

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// benchRun is one iteration's raw measurement: how long it took and
// how many bytes the Go runtime allocated while it ran (via
// runtime.MemStats.TotalAlloc deltas — cumulative-allocated, not
// currently-live, so it's unaffected by *when* a GC happens to run
// during or between iterations, the same technique `go test -bench
// -benchmem` itself uses). failed is whether that run's exit code was
// non-zero; still counted in every stat below (a slow, failing run is
// still real data about this program), just called out separately in
// the footer so a batch with a failure isn't mistaken for a clean one.
type benchRun struct {
	duration time.Duration
	allocB   uint64
	failed   bool
}

// benchResultMsg carries one full Bench-tab batch's result. err is an
// unreadable input file or an invalid/out-of-range run count — never
// a per-run failure, which shows up in runs[i].failed instead.
type benchResultMsg struct {
	runs []benchRun
	err  string
}

// maxBenchRuns caps how many runs a single batch can ask for.
// benchCmd runs synchronously inside one blocking tea.Cmd (the same
// shape runAllStoresCmd already uses) — bubbletea can't process a quit
// keypress, or re-enter Update at all, until that Cmd returns, so an
// accidental extra zero on the typed count (10000 instead of 1000)
// would otherwise hang the whole session with no way out until it
// finishes on its own.
const maxBenchRuns = 1000

// benchSeriesKind is which of the five toggleable lines a chart draws
// — shared by both the Runtime and Memory charts, so toggling one off
// hides it on both at once.
type benchSeriesKind int

const (
	benchRaw benchSeriesKind = iota
	benchAvg
	benchMedian
	benchMax
	benchMin
	benchSeriesKindCount
)

// benchSeriesInfo is one series' fixed display identity: its legend
// name, the key that toggles it (handleBenchTabKey), and its chart
// color. Order here is draw order too — raw first, so the reference
// lines paint over it (not under it) on any cell they both land on.
var benchSeriesInfo = [benchSeriesKindCount]struct {
	name  string
	key   rune
	color lipgloss.Color
}{
	benchRaw:    {"runs", 'r', colorTomato},
	benchAvg:    {"average", 'a', colorCheese},
	benchMedian: {"median", 'm', colorBasil},
	benchMax:    {"max", 'x', colorCrimson},
	benchMin:    {"min", 'n', colorSage},
}

// benchSeriesKeyOf returns which series r toggles, if any.
func benchSeriesKeyOf(r rune) (benchSeriesKind, bool) {
	for k, info := range benchSeriesInfo {
		if info.key == r {
			return benchSeriesKind(k), true
		}
	}
	return 0, false
}

// handleBenchTabKey routes keys on the Bench tab. The run-count field
// (like the Run tab's own input field) wants digits, backspace, and
// the usual cursor-movement keys; every toggle is a fixed, non-digit
// mnemonic (r/a/m/x/n, matching benchSeriesInfo's own key field) so
// there's no ambiguity between "type this into the field" and "toggle
// this series" the way there would be if the toggles were digits too.
func (m debugModel) handleBenchTabKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyTab:
		m.active = (m.active + 1) % tabCount
		m.maybeRefreshNav()
		return m, m.maybeOpenEditor()
	case tea.KeyShiftTab:
		m.active = (m.active + tabCount - 1) % tabCount
		m.maybeRefreshNav()
		return m, m.maybeOpenEditor()
	case tea.KeyCtrlC, tea.KeyEsc:
		return m, tea.Quit
	case tea.KeyEnter:
		return m, m.benchCmd()
	case tea.KeyLeft:
		m.benchCount.left()
	case tea.KeyRight:
		m.benchCount.right()
	case tea.KeyBackspace:
		m.benchCount.backspace()
	case tea.KeyDelete:
		m.benchCount.deleteForward()
	case tea.KeyHome, tea.KeyCtrlA:
		m.benchCount.cursor = 0
	case tea.KeyEnd, tea.KeyCtrlE:
		m.benchCount.cursor = len(m.benchCount.value)
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			switch {
			case r == 'q':
				return m, tea.Quit
			default:
				if kind, ok := benchSeriesKeyOf(r); ok {
					m.benchShow[kind] = !m.benchShow[kind]
				} else if r >= '0' && r <= '9' {
					m.benchCount.insert(r)
				}
			}
		}
	}
	return m, nil
}

// benchCmd reads the Run tab's current entry point and input file —
// the exact settings runProgramCmd itself would use — and runs the
// debugged file that many times in a row, each through the plain,
// untraced runFile. The input file (if any) is read once up front and
// reused for every iteration, the same reasoning runProgramCmd's own
// doc comment gives for its own single read: content can't ever
// differ between iterations, and it's the only way "no input file
// selected" and "read it once" share one code path
// (bytes.NewReader(nil) reads as immediate EOF, same as every
// iteration reusing the same bytes).
func (m debugModel) benchCmd() tea.Cmd {
	path, store := m.view.path, m.selectedRunEntry()
	inputPath := strings.TrimSpace(m.runInput.String())
	countText := strings.TrimSpace(m.benchCount.String())

	return func() tea.Msg {
		n, err := strconv.Atoi(countText)
		if err != nil || n <= 0 {
			return benchResultMsg{err: fmt.Sprintf("enter a positive number of runs, got %q", countText)}
		}
		if n > maxBenchRuns {
			return benchResultMsg{err: fmt.Sprintf("%d runs is more than the %d-run cap for this tab", n, maxBenchRuns)}
		}

		var data []byte
		if inputPath != "" {
			d, err := os.ReadFile(inputPath)
			if err != nil {
				return benchResultMsg{err: err.Error()}
			}
			data = d
		}

		saveBenchCountBestEffort(path, n)

		runs := make([]benchRun, n)
		for i := 0; i < n; i++ {
			var stdout, stderr bytes.Buffer
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			start := time.Now()
			code := runFile(path, store, bytes.NewReader(data), &stdout, &stderr)
			dur := time.Since(start)
			runtime.ReadMemStats(&after)
			runs[i] = benchRun{duration: dur, allocB: after.TotalAlloc - before.TotalAlloc, failed: code != 0}
		}
		return benchResultMsg{runs: runs}
	}
}

// handleBenchResult applies one Bench-tab batch's result. A failure
// (bad run count, unreadable input file) leaves the previous batch's
// results in place — the same "don't blank out still-valid data over
// a failed retry" choice every other tab's own result handler makes.
func (m debugModel) handleBenchResult(msg benchResultMsg) (tea.Model, tea.Cmd) {
	if msg.err != "" {
		m.benchErr = msg.err
		return m, nil
	}
	m.benchErr = ""
	m.benchRuns = msg.runs
	return m, nil
}

// --- stats ----------------------------------------------------------------

// benchOrdered is the numeric family every Bench-tab stat operates
// over: time.Duration (~int64) for the Runtime chart, uint64 for the
// Memory chart's byte counts — different underlying types, identical
// arithmetic, so one generic implementation covers both instead of
// two near-duplicate copies.
type benchOrdered interface{ ~int64 | ~uint64 }

func benchSum[T benchOrdered](xs []T) T {
	var sum T
	for _, x := range xs {
		sum += x
	}
	return sum
}

// benchAvgOf is the mean, truncated to T's own integer precision —
// acceptable here (nanoseconds or bytes), the same rounding a real
// terminal display would clip anyway.
func benchAvgOf[T benchOrdered](xs []T) T {
	if len(xs) == 0 {
		return 0
	}
	return benchSum(xs) / T(len(xs))
}

// benchMedianOf sorts a copy (never the caller's own slice) and
// averages the two middle values on an even count, the standard
// definition — more resistant than the mean to one slow outlier run
// (a stray GC pause, a scheduler hiccup) skewing the whole picture,
// which is exactly the "other stat that would be notable" a
// benchmark-style tool typically reaches for first.
func benchMedianOf[T benchOrdered](xs []T) T {
	if len(xs) == 0 {
		return 0
	}
	sorted := slices.Clone(xs)
	slices.Sort(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

func benchMaxOf[T benchOrdered](xs []T) T {
	if len(xs) == 0 {
		return 0
	}
	return slices.Max(xs)
}

func benchMinOf[T benchOrdered](xs []T) T {
	if len(xs) == 0 {
		return 0
	}
	return slices.Min(xs)
}

// --- chart ------------------------------------------------------------

// resampleForChart maps xs (one value per run) onto exactly width
// columns, however their counts compare: bucketAverage downsamples
// when there are more runs than columns (the compress direction), and
// this stretches xs across the full width via linear interpolation
// when there are fewer (a typical small run count against a typical
// wide terminal) — deliberately *not* just bucketAverage(xs, width)
// on its own, which would return only min(width, n) points and leave
// them crammed against the chart's left edge instead of spanning it,
// at a different effective horizontal scale than the reference lines
// (avg/median/max/min), which always span the full width — a real bug
// caught by actually looking at a rendered chart via a pty session,
// not by any unit test, since every existing test asserted on line
// *count*/*content*, never on where within the width they landed.
// n == 1 is its own case (interpolating between one point and itself
// is meaningless) — repeat that single value across every column
// instead.
func resampleForChart(xs []float64, width int) []float64 {
	n := len(xs)
	if n == 0 || width <= 0 {
		return nil
	}
	if n == 1 {
		out := make([]float64, width)
		for i := range out {
			out[i] = xs[0]
		}
		return out
	}
	if n >= width {
		return bucketAverage(xs, width)
	}
	out := make([]float64, width)
	for c := 0; c < width; c++ {
		pos := float64(c) * float64(n-1) / float64(width-1)
		lo := int(pos)
		hi := min(lo+1, n-1)
		frac := pos - float64(lo)
		out[c] = xs[lo]*(1-frac) + xs[hi]*frac
	}
	return out
}

// bucketAverage maps xs (typically one value per run) onto at most
// buckets columns, averaging whichever raw values land in each —
// exact per-run values with no averaging at all when len(xs) <=
// buckets (the common case: a chart usually has more columns than a
// sane run count), downsampled evenly when there are more runs than
// there is horizontal room to show them one-for-one. lo/hi's
// b*n/buckets split is the standard "distribute n items into buckets
// groups as evenly as integer division allows" trick — every bucket
// gets floor(n/buckets) or ceil(n/buckets) items, never zero, so no
// bucket goes empty and needs to fall back to copying its neighbor.
func bucketAverage(xs []float64, buckets int) []float64 {
	n := len(xs)
	if n == 0 || buckets <= 0 {
		return nil
	}
	if buckets > n {
		buckets = n
	}
	out := make([]float64, buckets)
	for b := 0; b < buckets; b++ {
		lo := b * n / buckets
		hi := (b + 1) * n / buckets
		sum := 0.0
		for i := lo; i < hi; i++ {
			sum += xs[i]
		}
		out[b] = sum / float64(hi-lo)
	}
	return out
}

// benchChart renders one metric's mini line chart: raw (one value per
// run, resampled onto the chart's full width via resampleForChart) as
// a solid stepped line, plus a flat dashed line per shown reference
// value in refs (indexed by benchSeriesKind — refs[benchRaw] is
// unused and always ignored, since raw isn't a single flat value).
// format renders one value for the two y-axis labels (top = the
// range's max, bottom = its min) in whichever unit the caller's
// metric uses (a Duration string, or formatBytes).
//
// The value range only stretches to cover series that are actually
// shown (show[kind] true) — a toggled-off reference line shouldn't
// widen the axis around a value nobody's looking at, which would also
// visually compress every series that *is* still shown for no reason
// the reader could see.
func benchChart(height, width int, raw []float64, refs [benchSeriesKindCount]float64, show [benchSeriesKindCount]bool, format func(float64) string) string {
	if height < 2 {
		height = 2
	}
	if width < 4 {
		width = 4
	}

	var values []float64
	if show[benchRaw] {
		values = append(values, raw...)
	}
	for _, kind := range []benchSeriesKind{benchAvg, benchMedian, benchMax, benchMin} {
		if show[kind] {
			values = append(values, refs[kind])
		}
	}
	if len(values) == 0 {
		return styleMuted.Render("(nothing to show — press a/m/x/n/r to enable a line)")
	}

	yMin, yMax := values[0], values[0]
	for _, v := range values {
		yMin = math.Min(yMin, v)
		yMax = math.Max(yMax, v)
	}
	if yMax == yMin {
		yMax = yMin + 1
	}

	rowFor := func(v float64) int {
		frac := (v - yMin) / (yMax - yMin)
		row := int(math.Round(frac * float64(height-1)))
		return max(0, min(height-1, row))
	}

	grid := make([][]rune, height)
	colorAt := make([][]lipgloss.Color, height)
	for r := range grid {
		grid[r] = make([]rune, width)
		colorAt[r] = make([]lipgloss.Color, width)
		for c := range grid[r] {
			grid[r][c] = ' '
		}
	}
	// plot writes into row height-1-row so row 0 (the smallest value)
	// lands on the grid's last (bottommost) printed line.
	plot := func(row, col int, ch rune, color lipgloss.Color) {
		if col < 0 || col >= width {
			return
		}
		gr := height - 1 - row
		grid[gr][col] = ch
		colorAt[gr][col] = color
	}

	if show[benchRaw] && len(raw) > 0 {
		bucketed := resampleForChart(raw, width)
		color := benchSeriesInfo[benchRaw].color
		prevRow := rowFor(bucketed[0])
		plot(prevRow, 0, '●', color)
		for c := 1; c < len(bucketed); c++ {
			row := rowFor(bucketed[c])
			lo, hi := min(prevRow, row), max(prevRow, row)
			for r := lo; r <= hi; r++ {
				plot(r, c, '│', color)
			}
			plot(row, c, '●', color)
			prevRow = row
		}
	}
	// Reference lines paint over raw wherever they overlap it, in a
	// fixed order matching benchSeriesInfo's own declared order —
	// deterministic regardless of which subset happens to be toggled
	// on, rather than depending on map iteration order.
	for _, kind := range []benchSeriesKind{benchAvg, benchMedian, benchMax, benchMin} {
		if !show[kind] {
			continue
		}
		row := rowFor(refs[kind])
		color := benchSeriesInfo[kind].color
		for c := 0; c < width; c += 2 {
			plot(row, c, '·', color)
		}
	}

	labelWidth := max(len(format(yMax)), len(format(yMin)))
	var b strings.Builder
	for r := 0; r < height; r++ {
		label := strings.Repeat(" ", labelWidth)
		switch r {
		case 0:
			label = fmt.Sprintf("%*s", labelWidth, format(yMax))
		case height - 1:
			label = fmt.Sprintf("%*s", labelWidth, format(yMin))
		}
		b.WriteString(styleMuted.Render(label))
		b.WriteByte(' ')
		for c := 0; c < width; c++ {
			ch := grid[r][c]
			if ch == ' ' {
				b.WriteRune(ch)
				continue
			}
			b.WriteString(lipgloss.NewStyle().Foreground(colorAt[r][c]).Render(string(ch)))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// formatBytes renders v (bytes) in whichever binary unit keeps it a
// short, readable number — B under 1KiB, otherwise KiB/MiB/GiB, one
// decimal place past B, matching this project's other "human-readable
// enough for a debug view" formatting (e.g. sizeText, debug_view.go).
func formatBytes(v float64) string {
	units := []string{"B", "KiB", "MiB", "GiB"}
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f%s", v, units[i])
	}
	return fmt.Sprintf("%.1f%s", v, units[i])
}

// --- view ---------------------------------------------------------------

// benchChartHeight/benchChartWidth size each of the two mini charts
// off the window's own dimensions, the same "derive from m.width/
// m.height, clamp to a sane floor" shape stepperBodyHeight already
// uses — 10 lines reserved above/between/below the two charts (the
// run-count field, its blank line, two titles, a blank line between
// charts, the legend, and the failure-count footer), split evenly
// between Runtime and Memory since neither chart is more important
// than the other. 12 columns reserved on the left of each chart for
// its own y-axis labels.
func (m debugModel) benchChartHeight() int {
	return max(4, (m.height-10)/2)
}

func (m debugModel) benchChartWidth() int {
	return max(10, m.width-12)
}

// viewBench is the Bench tab: the run-count field, then either
// instructions (nothing benchmarked yet) or the two charts plus their
// shared legend.
func (m debugModel) viewBench() string {
	var b strings.Builder
	b.WriteString(styleTitle.Render("runs: "))
	b.WriteString(m.benchCount.render())
	b.WriteString("\n\n")

	if m.benchErr != "" {
		b.WriteString(styleError.Render(m.benchErr))
		b.WriteString("\n\n")
	}

	if len(m.benchRuns) == 0 {
		b.WriteString(styleMuted.Render(fmt.Sprintf(
			"enter: run %s that many times with the Run tab's current entry point and input file, and chart the results here",
			m.view.path)))
		return b.String()
	}

	durations := make([]time.Duration, len(m.benchRuns))
	allocs := make([]uint64, len(m.benchRuns))
	failed := 0
	for i, r := range m.benchRuns {
		durations[i] = r.duration
		allocs[i] = r.allocB
		if r.failed {
			failed++
		}
	}

	height, width := m.benchChartHeight(), m.benchChartWidth()

	rawDur := make([]float64, len(durations))
	for i, d := range durations {
		rawDur[i] = float64(d)
	}
	refsDur := [benchSeriesKindCount]float64{
		benchAvg:    float64(benchAvgOf(durations)),
		benchMedian: float64(benchMedianOf(durations)),
		benchMax:    float64(benchMaxOf(durations)),
		benchMin:    float64(benchMinOf(durations)),
	}
	formatDur := func(v float64) string { return time.Duration(v).String() }

	b.WriteString(styleTitle.Render(fmt.Sprintf("runtime (%d runs)", len(m.benchRuns))))
	b.WriteByte('\n')
	b.WriteString(benchChart(height, width, rawDur, refsDur, m.benchShow, formatDur))
	b.WriteByte('\n')

	rawMem := make([]float64, len(allocs))
	for i, a := range allocs {
		rawMem[i] = float64(a)
	}
	refsMem := [benchSeriesKindCount]float64{
		benchAvg:    float64(benchAvgOf(allocs)),
		benchMedian: float64(benchMedianOf(allocs)),
		benchMax:    float64(benchMaxOf(allocs)),
		benchMin:    float64(benchMinOf(allocs)),
	}

	b.WriteString(styleTitle.Render("memory allocated"))
	b.WriteByte('\n')
	b.WriteString(benchChart(height, width, rawMem, refsMem, m.benchShow, formatBytes))
	b.WriteByte('\n')

	b.WriteString(m.viewBenchLegend())
	if failed > 0 {
		b.WriteString(styleError.Render(fmt.Sprintf("\n%d/%d runs exited non-zero", failed, len(m.benchRuns))))
	}
	return b.String()
}

// viewBenchLegend draws the five toggles as a row of labeled swatches
// — on in the series' own chart color, off dimmed — the same
// on/off-by-color convention the Run tab's own entry-point selector
// already uses (renderEntryOptions), so a glance tells you which
// lines are actually contributing to the charts above.
func (m debugModel) viewBenchLegend() string {
	parts := make([]string, 0, benchSeriesKindCount)
	for k, info := range benchSeriesInfo {
		kind := benchSeriesKind(k)
		state, style := "off", styleFaint
		if m.benchShow[kind] {
			state, style = "on", lipgloss.NewStyle().Foreground(info.color)
		}
		parts = append(parts, style.Render(fmt.Sprintf("[%c] %s (%s)", info.key, info.name, state)))
	}
	return strings.Join(parts, "   ")
}

// benchCountInput builds a runInputModel pre-filled with n's decimal
// digits, cursor at the end — the same shape restoreRunInput already
// builds for the Run tab's own remembered input-file path, just from
// an int (restoreBenchCount's return) instead of a string.
func benchCountInput(n int) runInputModel {
	value := []rune(strconv.Itoa(n))
	return runInputModel{value: value, cursor: len(value)}
}

// restoreBenchCount returns path's remembered run count, or 10 — a
// reasonable first-try sample size — if nothing's been remembered yet
// (including a corrupted or zero/negative stored value, which
// shouldn't be possible via saveBenchCountBestEffort but is still
// worth guarding against a hand-edited state file).
func restoreBenchCount(path string) int {
	abs, err := filepath.Abs(path)
	if err != nil {
		return 10
	}
	saved, ok := loadDevelState()[abs]
	if !ok || saved.BenchCount <= 0 {
		return 10
	}
	return saved.BenchCount
}

// saveBenchCountBestEffort persists path's last-used run count,
// preserving whatever else was already remembered for it (store,
// input, run-all) — best-effort, the same "a write failure shouldn't
// interrupt the run itself" reasoning saveDevelStateBestEffort's own
// doc comment gives.
func saveBenchCountBestEffort(path string, n int) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	all := loadDevelState()
	s := all[abs]
	s.BenchCount = n
	_ = saveDevelState(abs, s)
}
