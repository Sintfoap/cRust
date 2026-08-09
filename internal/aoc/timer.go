package aoc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// timerDir resolves to os.UserConfigDir() normally; tests override it
// to a temp directory. Deliberately a separate var from sessionDir
// (even though both default to the same func) so a test can override
// one without affecting the other.
var timerDir = os.UserConfigDir

// timerPath is the single JSON file every day's timer state lives in,
// keyed by "<year>/<day>" — same one-shared-file shape as
// cmd/crust/debug_state.go's develop_state.json, for the same reason:
// one file crust itself manages beats one dotfile scattered per day.
func timerPath() (string, error) {
	dir, err := timerDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "crust", "timers.json"), nil
}

// dayTimer is one day's timer record. Started is when the clock began
// running (StartTimer, or first auto-start on fetch); Elapsed is time
// already banked from previous start/stop cycles. A day currently
// running has Started non-zero; a stopped or never-started day has it
// zero. Total elapsed for a running day is Elapsed plus
// time.Since(Started); for a stopped day it's just Elapsed.
type dayTimer struct {
	Started time.Time     `json:"started,omitempty"`
	Elapsed time.Duration `json:"elapsed,omitempty"`
}

// timerKey formats the timers.json map key for a given year/day.
func timerKey(year, day int) string {
	return fmt.Sprintf("%d/%d", year, day)
}

func loadTimers() map[string]dayTimer {
	path, err := timerPath()
	if err != nil {
		return map[string]dayTimer{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]dayTimer{}
	}
	var m map[string]dayTimer
	if json.Unmarshal(data, &m) != nil || m == nil {
		return map[string]dayTimer{}
	}
	return m
}

// saveTimer writes t as year/day's new timer record, preserving every
// other day's record — the same read-mutate-write-whole-file shape
// cmd/crust/debug_state.go's saveDevelState uses, and for the same
// atomicity reason: a temp file plus rename means a crash mid-write
// can never leave timers.json corrupted for every other day too.
func saveTimer(year, day int, t dayTimer) error {
	path, err := timerPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	m := loadTimers()
	m[timerKey(year, day)] = t

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// StartTimer begins (or resumes) the clock for year/day, if it isn't
// already running. Starting an already-running day is a silent no-op
// — calling it repeatedly (e.g. every time `crust develop` opens the
// same file) never resets progress already banked.
func StartTimer(year, day int) error {
	m := loadTimers()
	t := m[timerKey(year, day)]
	if !t.Started.IsZero() {
		return nil
	}
	t.Started = time.Now()
	return saveTimer(year, day, t)
}

// StopTimer stops year/day's clock, if running, banking the elapsed
// time into Elapsed. Stopping an already-stopped (or never-started)
// day is a silent no-op.
func StopTimer(year, day int) error {
	m := loadTimers()
	t := m[timerKey(year, day)]
	if t.Started.IsZero() {
		return nil
	}
	t.Elapsed += time.Since(t.Started)
	t.Started = time.Time{}
	return saveTimer(year, day, t)
}

// Elapsed returns year/day's total tracked time — banked time plus,
// if the clock is currently running, time since it started — and
// whether the clock is running right now. A day with no record at all
// reads as zero elapsed, not running.
func Elapsed(year, day int) (elapsed time.Duration, running bool) {
	t := loadTimers()[timerKey(year, day)]
	if t.Started.IsZero() {
		return t.Elapsed, false
	}
	return t.Elapsed + time.Since(t.Started), true
}
