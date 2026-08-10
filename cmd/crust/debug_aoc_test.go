package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sintfoap/cRust/internal/aoc"
)

func TestDayNumberFromPath(t *testing.T) {
	tests := []struct {
		path    string
		wantDay int
		wantOk  bool
	}{
		{"day01.crust", 1, true},
		{"day1.crust", 1, true},
		{"Day06.crust", 6, true},
		{"day25.crust", 25, true},
		{"day1_essentials.crust", 1, true},
		{"/a/b/day09.crust", 9, true},
		{"dayNN_template.crust", 0, false},
		{"day26.crust", 0, false}, // out of AoC's 1-25 range
		{"day00.crust", 0, false},
		{"notes.crust", 0, false},
		{"scratch.crust", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			day, ok := dayNumberFromPath(tt.path)
			if ok != tt.wantOk || day != tt.wantDay {
				t.Errorf("dayNumberFromPath(%q) = (%d, %v), want (%d, %v)", tt.path, day, ok, tt.wantDay, tt.wantOk)
			}
		})
	}
}

func TestAocInputPath(t *testing.T) {
	got := aocInputPath("/x/y/day01.crust", 1)
	want := filepath.Join("/x/y", "day01_input.txt")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	got = aocInputPath("/x/y/day7.crust", 7)
	want = filepath.Join("/x/y", "day07_input.txt")
	if got != want {
		t.Errorf("got %q, want %q (always zero-padded)", got, want)
	}
}

func TestMaybeAutoFetchInputSkipsWithoutSession(t *testing.T) {
	withTempConfigHome(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "day01.crust")

	var stderr bytes.Buffer
	maybeAutoFetchInput(path, 2026, &stderr)

	if _, err := os.Stat(aocInputPath(path, 1)); err == nil {
		t.Error("expected no input file written with no session configured")
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want silence when no session is configured", stderr.String())
	}
}

func TestMaybeAutoFetchInputSkipsForNonDayFile(t *testing.T) {
	withTempConfigHome(t)
	if err := aoc.SaveSession("s"); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "scratch.crust")

	var stderr bytes.Buffer
	maybeAutoFetchInput(path, 2026, &stderr)
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want silence for a file with no day number", stderr.String())
	}
}

func TestMaybeAutoFetchInputFetchesAndStartsTimer(t *testing.T) {
	withTempConfigHome(t)
	if err := aoc.SaveSession("s"); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("puzzle input\n"))
	}))
	defer srv.Close()
	old := fetchBaseURL
	fetchBaseURL = srv.URL
	t.Cleanup(func() { fetchBaseURL = old })

	dir := t.TempDir()
	path := filepath.Join(dir, "day03.crust")

	var stderr bytes.Buffer
	maybeAutoFetchInput(path, 2026, &stderr)

	data, err := os.ReadFile(aocInputPath(path, 3))
	if err != nil {
		t.Fatalf("expected input to be written: %v", err)
	}
	if string(data) != "puzzle input\n" {
		t.Errorf("got %q", data)
	}
	if !strings.Contains(stderr.String(), "day 3") {
		t.Errorf("stderr = %q, want a confirmation mentioning day 3", stderr.String())
	}

	_, running := aoc.Elapsed(2026, 3)
	if !running {
		t.Error("expected auto-fetch to start the timer")
	}
}

// TestMaybeAutoFetchInputUsesThePassedYearNotHardcoded2026 confirms the
// year threading actually works: a past-year fetch/timer lands on that
// year, not on whatever defaultAoCYear happens to be — the whole point
// of `crust develop --year`.
func TestMaybeAutoFetchInputUsesThePassedYearNotHardcoded2026(t *testing.T) {
	withTempConfigHome(t)
	if err := aoc.SaveSession("s"); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte("2020 puzzle input\n"))
	}))
	defer srv.Close()
	old := fetchBaseURL
	fetchBaseURL = srv.URL
	t.Cleanup(func() { fetchBaseURL = old })

	dir := t.TempDir()
	path := filepath.Join(dir, "day03.crust")

	var stderr bytes.Buffer
	maybeAutoFetchInput(path, 2020, &stderr)

	if gotPath != "/2020/day/3/input" {
		t.Errorf("fetched path = %q, want /2020/day/3/input", gotPath)
	}
	if !strings.Contains(stderr.String(), "day 3, 2020") {
		t.Errorf("stderr = %q, want it to mention day 3, 2020", stderr.String())
	}

	if _, running := aoc.Elapsed(2020, 3); !running {
		t.Error("expected the timer to start under year 2020")
	}
	if _, running := aoc.Elapsed(2026, 3); running {
		t.Error("expected no timer started under defaultAoCYear (2026) when --year 2020 was requested")
	}
}

func TestMaybeAutoFetchInputNeverOverwritesExistingInput(t *testing.T) {
	withTempConfigHome(t)
	if err := aoc.SaveSession("s"); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fresh from the server\n"))
	}))
	defer srv.Close()
	old := fetchBaseURL
	fetchBaseURL = srv.URL
	t.Cleanup(func() { fetchBaseURL = old })

	dir := t.TempDir()
	path := filepath.Join(dir, "day04.crust")
	inputPath := aocInputPath(path, 4)
	if err := os.WriteFile(inputPath, []byte("hand-edited input\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	maybeAutoFetchInput(path, 2026, &stderr)

	data, _ := os.ReadFile(inputPath)
	if string(data) != "hand-edited input\n" {
		t.Errorf("existing input file was overwritten: %q", data)
	}
}

// writeAocDebugFile writes a minimal, valid .crust source file at
// dir/name, for tests building an emptyDebugView around a specific
// dayNN.crust filename (dayNumberFromPath cares about the name, not
// the content).
func writeAocDebugFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("x = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAocStatusLineEmptyWithNoRecord(t *testing.T) {
	withTempConfigHome(t)
	path := writeAocDebugFile(t, t.TempDir(), "day01.crust")
	view, err := emptyDebugView(path)
	if err != nil {
		t.Fatalf("emptyDebugView: %v", err)
	}
	m := newDebugModel(view)
	if got := m.aocStatusLine(); got != "" {
		t.Errorf("aocStatusLine() = %q, want empty with no timer record", got)
	}
}

func TestAocStatusLineShowsRunningTimer(t *testing.T) {
	withTempConfigHome(t)
	path := writeAocDebugFile(t, t.TempDir(), "day02.crust")
	if err := aoc.StartTimer(2026, 2); err != nil {
		t.Fatalf("StartTimer: %v", err)
	}

	view, err := emptyDebugView(path)
	if err != nil {
		t.Fatalf("emptyDebugView: %v", err)
	}
	m := newDebugModel(view)

	got := m.aocStatusLine()
	if !strings.Contains(got, "day 2") || !strings.Contains(got, "running") {
		t.Errorf("aocStatusLine() = %q, want it to mention day 2 and running", got)
	}
	if !m.aocTimerRunning() {
		t.Error("aocTimerRunning() = false, want true")
	}
}

func TestAocStatusLineShowsStoppedTimer(t *testing.T) {
	withTempConfigHome(t)
	path := writeAocDebugFile(t, t.TempDir(), "day05.crust")
	if err := aoc.StartTimer(2026, 5); err != nil {
		t.Fatalf("StartTimer: %v", err)
	}
	if err := aoc.StopTimer(2026, 5); err != nil {
		t.Fatalf("StopTimer: %v", err)
	}

	view, err := emptyDebugView(path)
	if err != nil {
		t.Fatalf("emptyDebugView: %v", err)
	}
	m := newDebugModel(view)

	got := m.aocStatusLine()
	if !strings.Contains(got, "stopped") {
		t.Errorf("aocStatusLine() = %q, want it to say stopped", got)
	}
	if m.aocTimerRunning() {
		t.Error("aocTimerRunning() = true, want false once stopped")
	}
}

func TestAocYearFallsBackToDefaultWhenUnset(t *testing.T) {
	m := debugModel{}
	if got := m.aocYear(); got != defaultAoCYear {
		t.Errorf("aocYear() = %d, want %d (fallback)", got, defaultAoCYear)
	}
}

func TestAocYearUsesOptsYearWhenSet(t *testing.T) {
	m := debugModel{opts: debugOptions{Year: 2020}}
	if got := m.aocYear(); got != 2020 {
		t.Errorf("aocYear() = %d, want 2020", got)
	}
}

// TestAocStatusLineUsesOptsYearNotHardcoded2026 confirms the header's
// stopwatch actually reads whichever year opts.Year resolved to,
// rather than always assuming defaultAoCYear the way it did before
// `crust develop --year` existed.
func TestAocStatusLineUsesOptsYearNotHardcoded2026(t *testing.T) {
	withTempConfigHome(t)
	path := writeAocDebugFile(t, t.TempDir(), "day02.crust")
	if err := aoc.StartTimer(2020, 2); err != nil {
		t.Fatalf("StartTimer: %v", err)
	}

	view, err := emptyDebugView(path)
	if err != nil {
		t.Fatalf("emptyDebugView: %v", err)
	}
	m := newDebugModel(view)
	m.opts.Year = 2020

	got := m.aocStatusLine()
	if !strings.Contains(got, "day 2, 2020") || !strings.Contains(got, "running") {
		t.Errorf("aocStatusLine() = %q, want it to mention day 2, 2020 and running", got)
	}
	if !m.aocTimerRunning() {
		t.Error("aocTimerRunning() = false, want true for the 2020 timer")
	}

	// A model still defaulted to defaultAoCYear should see nothing
	// running for this same day -- 2020's timer and 2026's are
	// genuinely independent records.
	m2 := newDebugModel(view)
	if m2.aocTimerRunning() {
		t.Error("aocTimerRunning() = true for defaultAoCYear, want false (the running timer is under 2020)")
	}
}

func TestDebugModelInitTicksOnlyWhenTimerRunning(t *testing.T) {
	withTempConfigHome(t)

	notRunningPath := writeAocDebugFile(t, t.TempDir(), "day09.crust")
	view, err := emptyDebugView(notRunningPath)
	if err != nil {
		t.Fatalf("emptyDebugView: %v", err)
	}
	m := newDebugModel(view)
	if cmd := m.Init(); cmd != nil {
		t.Error("Init() returned a tick command with no running timer")
	}

	runningPath := writeAocDebugFile(t, t.TempDir(), "day10.crust")
	if err := aoc.StartTimer(2026, 10); err != nil {
		t.Fatalf("StartTimer: %v", err)
	}
	view2, err := emptyDebugView(runningPath)
	if err != nil {
		t.Fatalf("emptyDebugView: %v", err)
	}
	m2 := newDebugModel(view2)
	if cmd := m2.Init(); cmd == nil {
		t.Error("Init() returned nil, want a tick command with a running timer")
	}
}

func TestDebugModelUpdateAocTickReschedulesWhileRunning(t *testing.T) {
	withTempConfigHome(t)
	path := writeAocDebugFile(t, t.TempDir(), "day11.crust")
	if err := aoc.StartTimer(2026, 11); err != nil {
		t.Fatalf("StartTimer: %v", err)
	}
	view, err := emptyDebugView(path)
	if err != nil {
		t.Fatalf("emptyDebugView: %v", err)
	}
	m := newDebugModel(view)

	_, cmd := m.Update(aocTickMsg{})
	if cmd == nil {
		t.Error("Update(aocTickMsg) returned nil cmd while timer still running, want a rescheduled tick")
	}

	if err := aoc.StopTimer(2026, 11); err != nil {
		t.Fatalf("StopTimer: %v", err)
	}
	_, cmd = m.Update(aocTickMsg{})
	if cmd != nil {
		t.Error("Update(aocTickMsg) returned a cmd after the timer stopped, want nil")
	}
}
