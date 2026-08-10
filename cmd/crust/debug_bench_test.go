package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// --- stats -----------------------------------------------------------

func TestBenchAvgOf(t *testing.T) {
	got := benchAvgOf([]time.Duration{1 * time.Second, 2 * time.Second, 3 * time.Second})
	if got != 2*time.Second {
		t.Errorf("benchAvgOf = %v, want 2s", got)
	}
}

func TestBenchAvgOfEmptyIsZero(t *testing.T) {
	if got := benchAvgOf[time.Duration](nil); got != 0 {
		t.Errorf("benchAvgOf(nil) = %v, want 0", got)
	}
}

func TestBenchMedianOfOddCount(t *testing.T) {
	got := benchMedianOf([]uint64{5, 1, 3})
	if got != 3 {
		t.Errorf("benchMedianOf = %d, want 3", got)
	}
}

func TestBenchMedianOfEvenCountAverages(t *testing.T) {
	got := benchMedianOf([]uint64{1, 2, 3, 4})
	if got != 2 { // (2+3)/2 = 2 (integer division)
		t.Errorf("benchMedianOf = %d, want 2", got)
	}
}

func TestBenchMedianDoesNotMutateInput(t *testing.T) {
	xs := []uint64{5, 1, 3}
	benchMedianOf(xs)
	if xs[0] != 5 || xs[1] != 1 || xs[2] != 3 {
		t.Errorf("input mutated: %v, want unchanged [5 1 3]", xs)
	}
}

func TestBenchMaxMinOf(t *testing.T) {
	xs := []uint64{5, 1, 3}
	if got := benchMaxOf(xs); got != 5 {
		t.Errorf("benchMaxOf = %d, want 5", got)
	}
	if got := benchMinOf(xs); got != 1 {
		t.Errorf("benchMinOf = %d, want 1", got)
	}
}

func TestBenchMaxMinOfEmptyIsZero(t *testing.T) {
	if got := benchMaxOf[uint64](nil); got != 0 {
		t.Errorf("benchMaxOf(nil) = %d, want 0", got)
	}
	if got := benchMinOf[uint64](nil); got != 0 {
		t.Errorf("benchMinOf(nil) = %d, want 0", got)
	}
}

// --- bucketAverage -----------------------------------------------------

func TestBucketAverageOneBucketPerValueWhenRoom(t *testing.T) {
	got := bucketAverage([]float64{10, 20, 30}, 5)
	want := []float64{10, 20, 30}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
			break
		}
	}
}

func TestBucketAverageDownsamplesWhenTooManyPoints(t *testing.T) {
	got := bucketAverage([]float64{0, 10, 20, 30}, 2)
	if len(got) != 2 {
		t.Fatalf("got %d buckets, want 2", len(got))
	}
	if got[0] != 5 { // avg(0,10)
		t.Errorf("bucket 0 = %v, want 5", got[0])
	}
	if got[1] != 25 { // avg(20,30)
		t.Errorf("bucket 1 = %v, want 25", got[1])
	}
}

func TestBucketAverageEmptyInputIsNil(t *testing.T) {
	if got := bucketAverage(nil, 5); got != nil {
		t.Errorf("bucketAverage(nil, 5) = %v, want nil", got)
	}
}

// --- resampleForChart ----------------------------------------------------

// TestResampleForChartStretchesFewerRunsAcrossFullWidth is the exact
// bug a pty session caught: with fewer runs than chart columns, the
// raw series must span the *whole* width (the same width the
// reference lines always span), not just its first len(xs) columns.
func TestResampleForChartStretchesFewerRunsAcrossFullWidth(t *testing.T) {
	got := resampleForChart([]float64{0, 10}, 11)
	if len(got) != 11 {
		t.Fatalf("got %d points, want 11 (the full width)", len(got))
	}
	if got[0] != 0 {
		t.Errorf("got[0] = %v, want 0 (the first run's own value)", got[0])
	}
	if got[10] != 10 {
		t.Errorf("got[10] = %v, want 10 (the last run's own value)", got[10])
	}
	if got[5] != 5 {
		t.Errorf("got[5] = %v, want 5 (linearly interpolated midpoint)", got[5])
	}
}

func TestResampleForChartSingleRunFillsTheWholeWidth(t *testing.T) {
	got := resampleForChart([]float64{7}, 5)
	if len(got) != 5 {
		t.Fatalf("got %d points, want 5", len(got))
	}
	for i, v := range got {
		if v != 7 {
			t.Errorf("got[%d] = %v, want 7 repeated across every column", i, v)
		}
	}
}

func TestResampleForChartDownsamplesMoreRunsThanWidth(t *testing.T) {
	got := resampleForChart([]float64{0, 10, 20, 30}, 2)
	if len(got) != 2 {
		t.Fatalf("got %d points, want 2 (downsampled to the width)", len(got))
	}
}

func TestResampleForChartEmptyInputIsNil(t *testing.T) {
	if got := resampleForChart(nil, 5); got != nil {
		t.Errorf("resampleForChart(nil, 5) = %v, want nil", got)
	}
}

// --- formatBytes ---------------------------------------------------------

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		v    float64
		want string
	}{
		{500, "500B"},
		{1536, "1.5KiB"},
		{1048576 * 2.5, "2.5MiB"},
	}
	for _, tt := range tests {
		if got := formatBytes(tt.v); got != tt.want {
			t.Errorf("formatBytes(%v) = %q, want %q", tt.v, got, tt.want)
		}
	}
}

// --- columnForRun ----------------------------------------------------------

func TestColumnForRunSingleRunCentersColumn(t *testing.T) {
	if got := columnForRun(0, 1, 20); got != 10 {
		t.Errorf("columnForRun(0, 1, 20) = %d, want 10 (centered)", got)
	}
}

func TestColumnForRunMoreRunsThanWidthBucketsForward(t *testing.T) {
	// Mirrors bucketAverage's own bucket boundaries: with 100 runs
	// across 10 columns, run 0 falls in bucket 0, run 50 in bucket 5,
	// run 99 in the last bucket.
	if got := columnForRun(0, 100, 10); got != 0 {
		t.Errorf("columnForRun(0, 100, 10) = %d, want 0", got)
	}
	if got := columnForRun(50, 100, 10); got != 5 {
		t.Errorf("columnForRun(50, 100, 10) = %d, want 5", got)
	}
	if got := columnForRun(99, 100, 10); got != 9 {
		t.Errorf("columnForRun(99, 100, 10) = %d, want 9", got)
	}
}

func TestColumnForRunFewerRunsThanWidthSpreadsAcrossFullWidth(t *testing.T) {
	// Mirrors resampleForChart's own interpolation vertices: with 3
	// runs across 21 columns, run 0 is the first column, run 2 the
	// last, run 1 exactly in the middle.
	if got := columnForRun(0, 3, 21); got != 0 {
		t.Errorf("columnForRun(0, 3, 21) = %d, want 0", got)
	}
	if got := columnForRun(1, 3, 21); got != 10 {
		t.Errorf("columnForRun(1, 3, 21) = %d, want 10", got)
	}
	if got := columnForRun(2, 3, 21); got != 20 {
		t.Errorf("columnForRun(2, 3, 21) = %d, want 20", got)
	}
}

func TestColumnForRunEqualRunsAndWidthIsOneToOne(t *testing.T) {
	for i := 0; i < 10; i++ {
		if got := columnForRun(i, 10, 10); got != i {
			t.Errorf("columnForRun(%d, 10, 10) = %d, want %d", i, got, i)
		}
	}
}

// --- clampBenchCursor -------------------------------------------------------

func TestClampBenchCursor(t *testing.T) {
	tests := []struct {
		cursor, n, want int
	}{
		{cursor: 0, n: 5, want: 0},
		{cursor: 4, n: 5, want: 4},
		{cursor: 5, n: 5, want: 4},  // past the end clamps to the last run
		{cursor: -1, n: 5, want: 0}, // never negative
		{cursor: 3, n: 0, want: 0},  // no runs at all
	}
	for _, tt := range tests {
		if got := clampBenchCursor(tt.cursor, tt.n); got != tt.want {
			t.Errorf("clampBenchCursor(%d, %d) = %d, want %d", tt.cursor, tt.n, got, tt.want)
		}
	}
}

// --- chart ---------------------------------------------------------------

func TestBenchChartNothingShownIsAPlaceholder(t *testing.T) {
	var show [benchSeriesKindCount]bool // all false
	out := benchChart(6, 20, []float64{1, 2, 3}, [benchSeriesKindCount]float64{}, show, func(v float64) string { return "" }, -1)
	if !strings.Contains(out, "nothing to show") {
		t.Errorf("benchChart() = %q, want a placeholder", out)
	}
}

func TestBenchChartRendersRequestedHeight(t *testing.T) {
	show := [benchSeriesKindCount]bool{benchRaw: true, benchAvg: true}
	refs := [benchSeriesKindCount]float64{benchAvg: 2}
	out := benchChart(5, 20, []float64{1, 2, 3}, refs, show, func(v float64) string { return "x" }, -1)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 5 {
		t.Errorf("got %d lines, want 5 (the requested height)", len(lines))
	}
}

func TestBenchChartCursorAddsACaretRow(t *testing.T) {
	show := [benchSeriesKindCount]bool{benchRaw: true, benchAvg: true}
	refs := [benchSeriesKindCount]float64{benchAvg: 2}

	without := benchChart(5, 20, []float64{1, 2, 3}, refs, show, func(v float64) string { return "x" }, -1)
	withCursor := benchChart(5, 20, []float64{1, 2, 3}, refs, show, func(v float64) string { return "x" }, 3)

	withoutLines := strings.Split(strings.TrimRight(without, "\n"), "\n")
	withLines := strings.Split(strings.TrimRight(withCursor, "\n"), "\n")
	if len(withLines) != len(withoutLines)+1 {
		t.Fatalf("got %d lines with a cursor, want %d (one extra caret row)", len(withLines), len(withoutLines)+1)
	}
	if !strings.Contains(withLines[len(withLines)-1], "^") {
		t.Errorf("last line = %q, want it to contain the cursor caret", withLines[len(withLines)-1])
	}
}

func TestBenchChartNoCursorAddsNoCaretRow(t *testing.T) {
	show := [benchSeriesKindCount]bool{benchRaw: true}
	out := benchChart(5, 20, []float64{1, 2, 3}, [benchSeriesKindCount]float64{}, show, func(v float64) string { return "x" }, -1)
	if strings.Contains(out, "^") {
		t.Errorf("benchChart() with no cursor = %q, should not contain a caret", out)
	}
}

func TestBenchChartFlatDataStillRenders(t *testing.T) {
	// Every value identical -- yMax == yMin, which must not divide by
	// zero computing the row fraction.
	show := [benchSeriesKindCount]bool{benchRaw: true}
	out := benchChart(4, 10, []float64{5, 5, 5}, [benchSeriesKindCount]float64{}, show, func(v float64) string { return "5" }, -1)
	if out == "" {
		t.Error("expected non-empty output for flat data")
	}
}

// --- key handling ----------------------------------------------------

func TestHandleBenchTabKeyDigitsGoToCountField(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	next, _ := m.handleBenchTabKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("42")})
	got := next.(debugModel)
	if got.benchCount.String() != "42" {
		t.Errorf("benchCount = %q, want %q", got.benchCount.String(), "42")
	}
}

func TestHandleBenchTabKeyToggleLettersToggleSeriesNotTheField(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	if !m.benchShow[benchAvg] {
		t.Fatal("expected benchAvg to start enabled")
	}
	next, _ := m.handleBenchTabKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	got := next.(debugModel)
	if got.benchShow[benchAvg] {
		t.Error("expected 'a' to toggle benchAvg off")
	}
	if got.benchCount.String() != "" {
		t.Errorf("benchCount = %q, want unchanged (toggle letters must not be typed into the field)", got.benchCount.String())
	}
}

func TestHandleBenchTabKeyAllFiveTogglesFlip(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	for _, r := range "ramxn" {
		next, _ := m.handleBenchTabKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = next.(debugModel)
	}
	for k := range m.benchShow {
		if m.benchShow[k] {
			t.Errorf("series %d still enabled after toggling every mnemonic off", k)
		}
	}
}

func TestHandleBenchTabKeyIToggleInspectRequiresRuns(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	next, _ := m.handleBenchTabKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	got := next.(debugModel)
	if got.benchInspect {
		t.Error("'i' should be a no-op with no runs yet")
	}
}

func TestHandleBenchTabKeyITogglesInspect(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.benchRuns = []benchRun{{duration: time.Millisecond}, {duration: 2 * time.Millisecond}}

	next, _ := m.handleBenchTabKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	got := next.(debugModel)
	if !got.benchInspect {
		t.Fatal("'i' should turn inspect mode on")
	}

	next, _ = got.handleBenchTabKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	got = next.(debugModel)
	if got.benchInspect {
		t.Error("a second 'i' should turn inspect mode back off")
	}
}

func TestHandleBenchTabKeyHLStepTheCursor(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.benchRuns = []benchRun{{}, {}, {}}
	m.benchInspect = true
	m.benchCursor = 1

	next, _ := m.handleBenchTabKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	got := next.(debugModel)
	if got.benchCursor != 2 {
		t.Errorf("'l' cursor = %d, want 2", got.benchCursor)
	}

	next, _ = got.handleBenchTabKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	got = next.(debugModel)
	if got.benchCursor != 1 {
		t.Errorf("'h' cursor = %d, want 1", got.benchCursor)
	}
}

func TestHandleBenchTabKeyCursorClampsAtEnds(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.benchRuns = []benchRun{{}, {}}
	m.benchInspect = true
	m.benchCursor = 0

	next, _ := m.handleBenchTabKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	got := next.(debugModel)
	if got.benchCursor != 0 {
		t.Errorf("'h' at the first run = %d, want 0 (clamped)", got.benchCursor)
	}

	m.benchCursor = 1
	next, _ = m.handleBenchTabKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	got = next.(debugModel)
	if got.benchCursor != 1 {
		t.Errorf("'l' at the last run = %d, want 1 (clamped)", got.benchCursor)
	}
}

func TestHandleBenchTabKeyGAndShiftGJumpToEnds(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.benchRuns = []benchRun{{}, {}, {}, {}}
	m.benchInspect = true
	m.benchCursor = 2

	next, _ := m.handleBenchTabKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	got := next.(debugModel)
	if got.benchCursor != 3 {
		t.Errorf("'G' cursor = %d, want 3 (last run)", got.benchCursor)
	}

	next, _ = got.handleBenchTabKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	got = next.(debugModel)
	if got.benchCursor != 0 {
		t.Errorf("'g' cursor = %d, want 0 (first run)", got.benchCursor)
	}
}

func TestHandleBenchTabKeyHLIgnoredOutsideInspectMode(t *testing.T) {
	// h/l must not do anything (and specifically must not go to the
	// count field, unlike digits) when inspect mode isn't active.
	m := newDebugModel(viewFor(t, "x = 1"))
	m.benchRuns = []benchRun{{}, {}, {}}
	next, _ := m.handleBenchTabKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	got := next.(debugModel)
	if got.benchCursor != 0 {
		t.Errorf("benchCursor = %d, want unchanged at 0", got.benchCursor)
	}
	if got.benchCount.String() != "" {
		t.Errorf("benchCount = %q, want unchanged ('l' is not a digit)", got.benchCount.String())
	}
}

func TestHandleBenchTabKeyQQuits(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	_, cmd := m.handleBenchTabKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("expected 'q' to return tea.Quit")
	}
	if msg := cmd(); msg != tea.Quit() {
		t.Errorf("cmd() = %v, want tea.Quit()", msg)
	}
}

func TestHandleBenchTabKeyBackspaceEditsField(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.benchCount.insert('5')
	m.benchCount.insert('0')
	next, _ := m.handleBenchTabKey(tea.KeyMsg{Type: tea.KeyBackspace})
	got := next.(debugModel)
	if got.benchCount.String() != "5" {
		t.Errorf("benchCount = %q, want %q", got.benchCount.String(), "5")
	}
}

func TestHandleBenchTabKeyTabSwitchesTabs(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabBench
	next, _ := m.handleBenchTabKey(tea.KeyMsg{Type: tea.KeyTab})
	got := next.(debugModel)
	if got.active != tabNav {
		t.Errorf("active = %v, want tabNav (Bench's own next tab)", got.active)
	}
}

func TestHandleBenchTabKeyShiftTabSwitchesTabs(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabBench
	next, _ := m.handleBenchTabKey(tea.KeyMsg{Type: tea.KeyShiftTab})
	got := next.(debugModel)
	if got.active != tabRun {
		t.Errorf("active = %v, want tabRun (Bench's own previous tab)", got.active)
	}
}

// --- benchCmd / handleBenchResult, real subprocess-equivalent --------

func TestBenchCmdRunsTheFileNTimes(t *testing.T) {
	withTempDevelStateDir(t)
	path := writeDebugFile(t, `deliver("hi")`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.benchCount.insert('3')

	msg := m.benchCmd()().(benchResultMsg)
	if msg.err != "" {
		t.Fatalf("benchCmd: %s", msg.err)
	}
	if len(msg.runs) != 3 {
		t.Fatalf("got %d runs, want 3", len(msg.runs))
	}
	for i, r := range msg.runs {
		if r.failed {
			t.Errorf("run %d: failed = true, want a clean exit", i)
		}
		if r.duration < 0 {
			t.Errorf("run %d: duration = %v, want >= 0", i, r.duration)
		}
	}
}

func TestBenchCmdInvalidCountIsError(t *testing.T) {
	withTempDevelStateDir(t)
	path := writeDebugFile(t, `deliver("hi")`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.benchCount.insert('0') // zero is not positive

	msg := m.benchCmd()().(benchResultMsg)
	if msg.err == "" {
		t.Error("expected an error for a zero run count")
	}
}

func TestBenchCmdNonNumericCountIsError(t *testing.T) {
	withTempDevelStateDir(t)
	path := writeDebugFile(t, `deliver("hi")`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.benchCount = runInputModel{} // empty field

	msg := m.benchCmd()().(benchResultMsg)
	if msg.err == "" {
		t.Error("expected an error for an empty run count")
	}
}

func TestBenchCmdOverCapIsError(t *testing.T) {
	withTempDevelStateDir(t)
	path := writeDebugFile(t, `deliver("hi")`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	for _, r := range "1001" {
		m.benchCount.insert(r)
	}

	msg := m.benchCmd()().(benchResultMsg)
	if msg.err == "" {
		t.Error("expected an error for a run count over the cap")
	}
}

func TestBenchCmdUnreadableInputFileIsError(t *testing.T) {
	withTempDevelStateDir(t)
	path := writeDebugFile(t, `deliver("hi")`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.benchCount.insert('1')
	for _, r := range "/no/such/input.txt" {
		m.runInput.insert(r)
	}

	msg := m.benchCmd()().(benchResultMsg)
	if msg.err == "" {
		t.Error("expected an error for an unreadable input file")
	}
}

func TestBenchCmdFlagsAFailingRun(t *testing.T) {
	withTempDevelStateDir(t)
	path := writeDebugFile(t, `nope(`) // parse error -> non-zero exit, but no panic
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.benchCount.insert('1')

	msg := m.benchCmd()().(benchResultMsg)
	if msg.err != "" {
		t.Fatalf("benchCmd: %s", msg.err)
	}
	if len(msg.runs) != 1 || !msg.runs[0].failed {
		t.Errorf("runs = %+v, want exactly one failed run", msg.runs)
	}
}

func TestHandleBenchResultSuccessStoresRuns(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	next, _ := m.handleBenchResult(benchResultMsg{runs: []benchRun{{duration: time.Millisecond}}})
	got := next.(debugModel)
	if len(got.benchRuns) != 1 {
		t.Errorf("benchRuns = %v, want 1 entry", got.benchRuns)
	}
	if got.benchErr != "" {
		t.Errorf("benchErr = %q, want cleared", got.benchErr)
	}
}

func TestHandleBenchResultErrorKeepsPreviousRuns(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.benchRuns = []benchRun{{duration: time.Millisecond}}
	next, _ := m.handleBenchResult(benchResultMsg{err: "boom"})
	got := next.(debugModel)
	if got.benchErr != "boom" {
		t.Errorf("benchErr = %q, want %q", got.benchErr, "boom")
	}
	if len(got.benchRuns) != 1 {
		t.Errorf("benchRuns = %v, want the previous batch left in place", got.benchRuns)
	}
}

func TestHandleBenchResultClampsCursorToNewRunCount(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.benchInspect = true
	m.benchCursor = 9                                                            // selected a run in a 10-run batch
	next, _ := m.handleBenchResult(benchResultMsg{runs: []benchRun{{}, {}, {}}}) // a re-run with only 3
	got := next.(debugModel)
	if got.benchCursor != 2 {
		t.Errorf("benchCursor after a smaller re-run = %d, want 2 (clamped to the new last run)", got.benchCursor)
	}
	if !got.benchInspect {
		t.Error("a new batch of results should not silently exit inspect mode")
	}
}

// --- CSV export --------------------------------------------------------

func TestBenchExportPathSwapsExtensionForBenchCSV(t *testing.T) {
	m := newDebugModel(&debugView{path: "/tmp/day01.crust", rec: viewFor(t, "x = 1").rec})
	if got, want := m.benchExportPath(), "/tmp/day01.bench.csv"; got != want {
		t.Errorf("benchExportPath() = %q, want %q", got, want)
	}
}

func TestBenchExportCmdWritesCSV(t *testing.T) {
	path := writeDebugFile(t, "x = 1")
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.benchRuns = []benchRun{
		{duration: 1 * time.Millisecond, allocB: 100, failed: false},
		{duration: 2 * time.Millisecond, allocB: 200, failed: true},
	}

	msg := m.benchExportCmd()().(benchExportMsg)
	if msg.err != "" {
		t.Fatalf("benchExportCmd: %s", msg.err)
	}
	if msg.path != m.benchExportPath() {
		t.Errorf("msg.path = %q, want %q", msg.path, m.benchExportPath())
	}

	data, err := os.ReadFile(msg.path)
	if err != nil {
		t.Fatalf("reading exported CSV: %s", err)
	}
	want := "run,duration_ns,alloc_bytes,failed\n" +
		"1,1000000,100,false\n" +
		"2,2000000,200,true\n"
	if string(data) != want {
		t.Errorf("CSV content = %q, want %q", string(data), want)
	}
}

func TestBenchExportCmdUnwritablePathIsError(t *testing.T) {
	m := newDebugModel(&debugView{path: "/nonexistent-dir-xyz/day01.crust", rec: viewFor(t, "x = 1").rec})
	m.benchRuns = []benchRun{{duration: time.Millisecond}}

	msg := m.benchExportCmd()().(benchExportMsg)
	if msg.err == "" {
		t.Error("expected an error writing to a nonexistent directory")
	}
}

func TestHandleBenchExportResultSuccess(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	next, _ := m.handleBenchExportResult(benchExportMsg{path: "/tmp/day01.bench.csv"})
	got := next.(debugModel)
	if !strings.Contains(got.benchExportStatus, "/tmp/day01.bench.csv") {
		t.Errorf("benchExportStatus = %q, want it to mention the exported path", got.benchExportStatus)
	}
}

func TestHandleBenchExportResultError(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	next, _ := m.handleBenchExportResult(benchExportMsg{err: "disk full"})
	got := next.(debugModel)
	if !strings.Contains(got.benchExportStatus, "disk full") {
		t.Errorf("benchExportStatus = %q, want it to mention the error", got.benchExportStatus)
	}
}

func TestHandleBenchResultClearsStaleExportStatus(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.benchExportStatus = "exported to /tmp/old.bench.csv"
	next, _ := m.handleBenchResult(benchResultMsg{runs: []benchRun{{duration: time.Millisecond}}})
	got := next.(debugModel)
	if got.benchExportStatus != "" {
		t.Errorf("benchExportStatus = %q, want cleared after a fresh batch", got.benchExportStatus)
	}
}

func TestHandleBenchTabKeyERequiresRuns(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	_, cmd := m.handleBenchTabKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if cmd != nil {
		t.Error("'e' with no runs yet should be a no-op, not return an export Cmd")
	}
}

func TestHandleBenchTabKeyEExportsWithRuns(t *testing.T) {
	path := writeDebugFile(t, "x = 1")
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.benchRuns = []benchRun{{duration: time.Millisecond, allocB: 10}}

	_, cmd := m.handleBenchTabKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if cmd == nil {
		t.Fatal("'e' with runs present should return an export Cmd")
	}
	msg := cmd().(benchExportMsg)
	if msg.err != "" {
		t.Fatalf("export failed: %s", msg.err)
	}
	if _, err := os.Stat(msg.path); err != nil {
		t.Errorf("exported file not found: %s", err)
	}
}

func TestViewBenchShowsExportStatus(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.width, m.height = 100, 40
	m.benchRuns = []benchRun{{duration: time.Millisecond, allocB: 10}}
	m.benchExportStatus = "exported to /tmp/day01.bench.csv"
	out := m.viewBench()
	if !strings.Contains(out, "exported to /tmp/day01.bench.csv") {
		t.Errorf("viewBench() missing export status: %q", out)
	}
}

// --- baseline diff -------------------------------------------------------

func TestBenchBaselineFromComputesAverages(t *testing.T) {
	runs := []benchRun{
		{duration: 1 * time.Millisecond, allocB: 100},
		{duration: 3 * time.Millisecond, allocB: 300},
	}
	got := benchBaselineFrom(runs)
	if got.DurationAvgNS != int64(2*time.Millisecond) {
		t.Errorf("DurationAvgNS = %d, want %d", got.DurationAvgNS, int64(2*time.Millisecond))
	}
	if got.AllocAvg != 200 {
		t.Errorf("AllocAvg = %d, want 200", got.AllocAvg)
	}
}

func TestBenchDiffPct(t *testing.T) {
	pct, ok := benchDiffPct(120, 100)
	if !ok {
		t.Fatal("expected ok=true for a nonzero baseline")
	}
	if pct != 20 {
		t.Errorf("benchDiffPct(120, 100) = %v, want 20", pct)
	}

	pct, ok = benchDiffPct(80, 100)
	if !ok || pct != -20 {
		t.Errorf("benchDiffPct(80, 100) = %v, %v, want -20, true", pct, ok)
	}
}

func TestBenchDiffPctZeroBaselineIsNotOK(t *testing.T) {
	_, ok := benchDiffPct(100, 0)
	if ok {
		t.Error("benchDiffPct with a zero baseline should report ok=false, not divide by zero")
	}
}

func TestRestoreBenchBaselineDefaultsToNil(t *testing.T) {
	withTempDevelStateDir(t)
	if got := restoreBenchBaseline("/some/path.crust"); got != nil {
		t.Errorf("restoreBenchBaseline (nothing saved) = %v, want nil", got)
	}
}

func TestSaveAndRestoreBenchBaselineRoundTrips(t *testing.T) {
	withTempDevelStateDir(t)
	path := writeDebugFile(t, "x = 1")
	saveBenchBaselineBestEffort(path, benchBaseline{DurationAvgNS: 500, AllocAvg: 1024})

	got := restoreBenchBaseline(path)
	if got == nil {
		t.Fatal("expected a saved baseline")
	}
	if got.DurationAvgNS != 500 || got.AllocAvg != 1024 {
		t.Errorf("restoreBenchBaseline() = %+v, want {500 1024}", got)
	}
}

func TestSaveBenchBaselinePreservesOtherSettings(t *testing.T) {
	withTempDevelStateDir(t)
	path := writeDebugFile(t, "x = 1")
	saveDevelStateBestEffort(path, "part1", "input.txt", true)
	saveBenchCountBestEffort(path, 25)
	saveBenchBaselineBestEffort(path, benchBaseline{DurationAvgNS: 500, AllocAvg: 1024})

	abs, _ := filepath.Abs(path)
	saved := loadDevelState()[abs]
	if saved.Store != "part1" || saved.Input != "input.txt" || !saved.RunAll || saved.BenchCount != 25 {
		t.Errorf("saved = %+v, want store/input/runAll/benchCount preserved alongside the new baseline", saved)
	}
	if saved.BenchBaseline == nil || saved.BenchBaseline.DurationAvgNS != 500 {
		t.Errorf("BenchBaseline = %+v, want {500 1024}", saved.BenchBaseline)
	}
}

func TestHandleBenchTabKeyBSavesBaseline(t *testing.T) {
	withTempDevelStateDir(t)
	path := writeDebugFile(t, "x = 1")
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.benchRuns = []benchRun{
		{duration: 1 * time.Millisecond, allocB: 100},
		{duration: 3 * time.Millisecond, allocB: 300},
	}

	next, _ := m.handleBenchTabKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	got := next.(debugModel)
	if got.benchBaseline == nil {
		t.Fatal("'b' should set benchBaseline")
	}
	if got.benchBaseline.DurationAvgNS != int64(2*time.Millisecond) {
		t.Errorf("DurationAvgNS = %d, want %d", got.benchBaseline.DurationAvgNS, int64(2*time.Millisecond))
	}
	if got.benchBaselineStatus == "" {
		t.Error("expected a non-empty benchBaselineStatus after saving")
	}

	// It should also actually be persisted, not just held in memory.
	if saved := restoreBenchBaseline(path); saved == nil || saved.DurationAvgNS != int64(2*time.Millisecond) {
		t.Errorf("restoreBenchBaseline() = %+v, want the saved baseline to persist", saved)
	}
}

func TestHandleBenchTabKeyBRequiresRuns(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	next, _ := m.handleBenchTabKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	got := next.(debugModel)
	if got.benchBaseline != nil {
		t.Error("'b' with no runs yet should be a no-op")
	}
}

func TestViewBenchBaselineShowsDiff(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.width, m.height = 100, 40
	m.benchRuns = []benchRun{
		{duration: 240 * time.Millisecond, allocB: 60},
	}
	m.benchBaseline = &benchBaseline{DurationAvgNS: int64(200 * time.Millisecond), AllocAvg: 50}

	out := m.viewBench()
	if !strings.Contains(out, "vs baseline") {
		t.Errorf("viewBench() missing the baseline diff: %q", out)
	}
	if !strings.Contains(out, "+20.0%") {
		t.Errorf("viewBench() = %q, want it to show +20.0%% (240ms vs a 200ms baseline)", out)
	}
}

func TestViewBenchBaselineHiddenWithoutASavedBaseline(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.width, m.height = 100, 40
	m.benchRuns = []benchRun{{duration: time.Millisecond, allocB: 10}}
	out := m.viewBench()
	if strings.Contains(out, "vs baseline") {
		t.Errorf("viewBench() = %q, should not show a baseline diff when none has been saved", out)
	}
}

func TestHandleBenchResultClearsBaselineStatusButKeepsBaseline(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.benchBaseline = &benchBaseline{DurationAvgNS: 500, AllocAvg: 1024}
	m.benchBaselineStatus = "baseline saved"

	next, _ := m.handleBenchResult(benchResultMsg{runs: []benchRun{{duration: time.Millisecond}}})
	got := next.(debugModel)
	if got.benchBaselineStatus != "" {
		t.Errorf("benchBaselineStatus = %q, want cleared after a fresh batch", got.benchBaselineStatus)
	}
	if got.benchBaseline == nil || got.benchBaseline.DurationAvgNS != 500 {
		t.Errorf("benchBaseline = %+v, want it to survive a fresh batch of runs", got.benchBaseline)
	}
}

// --- two-entry-point compare ---------------------------------------------

func TestBenchCompareSnapshotFromComputesAveragesAndLabel(t *testing.T) {
	runs := []benchRun{
		{duration: 1 * time.Millisecond, allocB: 100},
		{duration: 3 * time.Millisecond, allocB: 300},
	}
	got := benchCompareSnapshotFrom(runs, "part1")
	if got.EntryLabel != "part1" {
		t.Errorf("EntryLabel = %q, want %q", got.EntryLabel, "part1")
	}
	if got.Runs != 2 {
		t.Errorf("Runs = %d, want 2", got.Runs)
	}
	if got.DurationAvgNS != int64(2*time.Millisecond) {
		t.Errorf("DurationAvgNS = %d, want %d", got.DurationAvgNS, int64(2*time.Millisecond))
	}
	if got.AllocAvg != 200 {
		t.Errorf("AllocAvg = %d, want 200", got.AllocAvg)
	}
}

func TestBenchCompareSnapshotFromBareStoreLabelsAsDefault(t *testing.T) {
	got := benchCompareSnapshotFrom([]benchRun{{duration: time.Millisecond}}, "")
	if got.EntryLabel != "(default)" {
		t.Errorf("EntryLabel = %q, want %q for the bare store", got.EntryLabel, "(default)")
	}
}

func TestHandleBenchTabKeyCCapturesForCompare(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.benchRuns = []benchRun{
		{duration: 1 * time.Millisecond, allocB: 100},
		{duration: 3 * time.Millisecond, allocB: 300},
	}

	next, _ := m.handleBenchTabKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	got := next.(debugModel)
	if got.benchCompare == nil {
		t.Fatal("'c' should set benchCompare")
	}
	if got.benchCompare.DurationAvgNS != int64(2*time.Millisecond) {
		t.Errorf("DurationAvgNS = %d, want %d", got.benchCompare.DurationAvgNS, int64(2*time.Millisecond))
	}
	if got.benchCompare.Runs != 2 {
		t.Errorf("Runs = %d, want 2", got.benchCompare.Runs)
	}
	if got.benchCompareStatus == "" {
		t.Error("expected a non-empty benchCompareStatus after capturing")
	}
}

func TestHandleBenchTabKeyCRequiresRuns(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	next, _ := m.handleBenchTabKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	got := next.(debugModel)
	if got.benchCompare != nil {
		t.Error("'c' with no runs yet should be a no-op")
	}
}

func TestViewBenchCompareShowsBothLabelsAndDiff(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.width, m.height = 100, 40
	m.benchRuns = []benchRun{{duration: 240 * time.Millisecond, allocB: 60}}
	m.benchCompare = &benchCompareSnapshot{
		EntryLabel: "part1", Runs: 5,
		DurationAvgNS: int64(200 * time.Millisecond), AllocAvg: 50,
	}

	out := m.viewBench()
	if !strings.Contains(out, "compare:") {
		t.Errorf("viewBench() missing the compare line: %q", out)
	}
	if !strings.Contains(out, "part1 (5 runs)") {
		t.Errorf("viewBench() = %q, want the captured entry's label and run count shown", out)
	}
	if !strings.Contains(out, "(default) (1 runs)") {
		t.Errorf("viewBench() = %q, want the current (bare store) entry labeled (default)", out)
	}
	if !strings.Contains(out, "+20.0%") {
		t.Errorf("viewBench() = %q, want +20.0%% (240ms vs a 200ms captured average)", out)
	}
}

func TestViewBenchCompareHiddenWithoutACapture(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.width, m.height = 100, 40
	m.benchRuns = []benchRun{{duration: time.Millisecond, allocB: 10}}
	out := m.viewBench()
	if strings.Contains(out, "compare:") {
		t.Errorf("viewBench() = %q, should not show a compare line when nothing has been captured", out)
	}
}

func TestHandleBenchResultClearsCompareStatusButKeepsCompare(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.benchCompare = &benchCompareSnapshot{EntryLabel: "part1", Runs: 3, DurationAvgNS: 500, AllocAvg: 1024}
	m.benchCompareStatus = "captured part1 (3 runs) for comparison"

	next, _ := m.handleBenchResult(benchResultMsg{runs: []benchRun{{duration: time.Millisecond}}})
	got := next.(debugModel)
	if got.benchCompareStatus != "" {
		t.Errorf("benchCompareStatus = %q, want cleared after a fresh batch", got.benchCompareStatus)
	}
	if got.benchCompare == nil || got.benchCompare.EntryLabel != "part1" {
		t.Errorf("benchCompare = %+v, want it to survive a fresh batch of runs", got.benchCompare)
	}
}

func TestBenchCompareIsNilAtStartup(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	if m.benchCompare != nil {
		t.Error("expected benchCompare to start nil (session-only, never restored from disk)")
	}
}

// --- persistence -------------------------------------------------------

func TestRestoreBenchCountDefaultsToTen(t *testing.T) {
	withTempDevelStateDir(t)
	if got := restoreBenchCount("/some/path.crust"); got != 10 {
		t.Errorf("restoreBenchCount (nothing saved) = %d, want 10", got)
	}
}

func TestSaveAndRestoreBenchCountRoundTrips(t *testing.T) {
	withTempDevelStateDir(t)
	path := writeDebugFile(t, "x = 1")
	saveBenchCountBestEffort(path, 25)
	if got := restoreBenchCount(path); got != 25 {
		t.Errorf("restoreBenchCount = %d, want 25", got)
	}
}

func TestSaveBenchCountPreservesOtherSettings(t *testing.T) {
	withTempDevelStateDir(t)
	path := writeDebugFile(t, "x = 1")
	saveDevelStateBestEffort(path, "part1", "input.txt", true)
	saveBenchCountBestEffort(path, 25)

	abs, _ := filepath.Abs(path)
	saved := loadDevelState()[abs]
	if saved.Store != "part1" || saved.Input != "input.txt" || !saved.RunAll {
		t.Errorf("saved = %+v, want store/input/runAll preserved alongside the new BenchCount", saved)
	}
	if saved.BenchCount != 25 {
		t.Errorf("BenchCount = %d, want 25", saved.BenchCount)
	}
}

// --- view ----------------------------------------------------------------

func TestViewBenchNoRunsYetShowsInstructions(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	out := m.viewBench()
	if !strings.Contains(out, "enter: run") {
		t.Errorf("viewBench() = %q, want instructions", out)
	}
}

func TestViewBenchShowsError(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.benchErr = "something broke"
	out := m.viewBench()
	if !strings.Contains(out, "something broke") {
		t.Errorf("viewBench() = %q, want the error shown", out)
	}
}

func TestViewBenchWithResultsShowsBothCharts(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.width, m.height = 100, 40
	m.benchRuns = []benchRun{
		{duration: 1 * time.Millisecond, allocB: 100},
		{duration: 2 * time.Millisecond, allocB: 200},
		{duration: 3 * time.Millisecond, allocB: 300},
	}
	out := m.viewBench()
	if !strings.Contains(out, "runtime (3 runs)") {
		t.Errorf("viewBench() = %q, want a runtime heading", out)
	}
	if !strings.Contains(out, "memory allocated") {
		t.Errorf("viewBench() = %q, want a memory heading", out)
	}
	if !strings.Contains(out, "average") || !strings.Contains(out, "median") {
		t.Errorf("viewBench() = %q, want the legend", out)
	}
}

// TestViewBenchLegendShowsComputedValues is the exact feature asked
// for: "can you add values for the average and median lines?" --
// checks the legend prints the real computed numbers, in each chart's
// own unit (a Duration string for runtime, formatBytes for memory),
// not just the bare series name.
func TestViewBenchLegendShowsComputedValues(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.width, m.height = 100, 40
	m.benchRuns = []benchRun{
		{duration: 1 * time.Millisecond, allocB: 100},
		{duration: 2 * time.Millisecond, allocB: 200},
		{duration: 3 * time.Millisecond, allocB: 300},
	}
	out := m.viewBench()

	// avg = median = 2ms / 200B; max = 3ms / 300B; min = 1ms / 100B.
	for _, want := range []string{
		"average: 2ms", "median: 2ms", "max: 3ms", "min: 1ms",
		"average: 200B", "median: 200B", "max: 300B", "min: 100B",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("viewBench() missing %q; got:\n%s", want, out)
		}
	}
	if strings.Contains(out, "runs: 2ms") || strings.Contains(out, "runs: 200B") {
		t.Error("the raw 'runs' series has no single value and must not get one printed")
	}
}

func TestViewBenchInspectShowsSelectedRunStats(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.width, m.height = 100, 40
	m.benchRuns = []benchRun{
		{duration: 1 * time.Millisecond, allocB: 100},
		{duration: 2 * time.Millisecond, allocB: 200},
		{duration: 3 * time.Millisecond, allocB: 300},
	}
	m.benchInspect = true
	m.benchCursor = 1

	out := m.viewBench()
	if !strings.Contains(out, "run 2 of 3") {
		t.Errorf("viewBench() = %q, want it to name the selected run (1-based)", out)
	}
	if !strings.Contains(out, "2ms") || !strings.Contains(out, "200B") {
		t.Errorf("viewBench() = %q, want the selected run's own exact duration/memory", out)
	}
}

func TestViewBenchInspectHiddenWhenNotInspecting(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.width, m.height = 100, 40
	m.benchRuns = []benchRun{{duration: time.Millisecond, allocB: 100}}
	out := m.viewBench()
	if strings.Contains(out, "run 1 of 1") {
		t.Errorf("viewBench() = %q, should not show the inspect readout when benchInspect is false", out)
	}
}

func TestViewBenchInspectFlagsAFailedRun(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.width, m.height = 100, 40
	m.benchRuns = []benchRun{{duration: time.Millisecond, failed: true}}
	m.benchInspect = true
	m.benchCursor = 0
	out := m.viewBench()
	if !strings.Contains(out, "FAILED") {
		t.Errorf("viewBench() = %q, want the selected run's own failure flagged", out)
	}
}

func TestViewBenchReportsFailedRuns(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.width, m.height = 100, 40
	m.benchRuns = []benchRun{
		{duration: time.Millisecond, failed: true},
		{duration: time.Millisecond, failed: false},
	}
	out := m.viewBench()
	if !strings.Contains(out, "1/2 runs exited non-zero") {
		t.Errorf("viewBench() = %q, want the failure count noted", out)
	}
}

// --- overall wiring --------------------------------------------------

func TestViewTabsIncludesBenchLabel(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	out := m.viewTabs()
	if !strings.Contains(out, "Bench") {
		t.Errorf("viewTabs() = %q, want a Bench tab label", out)
	}
}

func TestHelpTextOnBenchTab(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabBench
	if !strings.Contains(m.helpText(), "toggle") {
		t.Errorf("helpText() = %q, want it to mention toggling a line", m.helpText())
	}
}

func TestViewRendersBenchTabWithoutPanicking(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.width, m.height = 80, 24
	m.active = tabBench
	m.benchRuns = []benchRun{{duration: time.Millisecond, allocB: 10}}
	out := m.View()
	if !strings.Contains(out, "runtime") {
		t.Errorf("View() = %q, want the Bench tab body rendered", out)
	}
}

func TestViewRendersBenchTabInspectModeWithoutPanicking(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.width, m.height = 80, 24
	m.active = tabBench
	m.benchRuns = []benchRun{{duration: time.Millisecond, allocB: 10}, {duration: 2 * time.Millisecond, allocB: 20}}
	m.benchInspect = true
	m.benchCursor = 1
	out := m.View()
	if !strings.Contains(out, "run 2 of 2") {
		t.Errorf("View() = %q, want the inspect readout rendered", out)
	}
}

func TestHelpTextOnBenchTabInspectMode(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabBench
	m.benchInspect = true
	help := m.helpText()
	if !strings.Contains(help, "prev/next run") {
		t.Errorf("helpText() = %q, want inspect-mode navigation hints", help)
	}
}

func TestNewDebugModelDefaultsAllBenchSeriesShown(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	for k := range m.benchShow {
		if !m.benchShow[k] {
			t.Errorf("series %d not shown by default", k)
		}
	}
}
