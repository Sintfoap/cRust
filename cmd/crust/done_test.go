package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Sintfoap/cRust/internal/aoc"
)

func TestParseDoneArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantDay  int
		wantYear int
		wantErr  bool
	}{
		{"day only", []string{"1"}, 1, defaultAoCYear, false},
		{"day with year", []string{"5", "--year", "2020"}, 5, 2020, false},
		{"day with year=", []string{"5", "--year=2020"}, 5, 2020, false},
		{"no args", nil, 0, 0, true},
		{"bad day", []string{"nope"}, 0, 0, true},
		{"unknown flag", []string{"1", "--bogus"}, 0, 0, true},
		{"two positional args", []string{"1", "2"}, 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			day, year, err := parseDoneArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseDoneArgs(%v) error = nil, want an error", tt.args)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDoneArgs(%v) error = %v, want nil", tt.args, err)
			}
			if day != tt.wantDay || year != tt.wantYear {
				t.Errorf("parseDoneArgs(%v) = (%d, %d), want (%d, %d)", tt.args, day, year, tt.wantDay, tt.wantYear)
			}
		})
	}
}

func TestRunDoneStopsRunningTimerAndPrintsElapsed(t *testing.T) {
	withTempConfigHome(t)

	if err := aoc.StartTimer(2026, 1); err != nil {
		t.Fatalf("StartTimer: %v", err)
	}
	time.Sleep(5 * time.Millisecond)

	var stdout, stderr bytes.Buffer
	code := runDone(1, 2026, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Day 1, 2026") {
		t.Errorf("stdout = %q, want it to mention the day", stdout.String())
	}

	_, running := aoc.Elapsed(2026, 1)
	if running {
		t.Error("expected the timer to be stopped after `crust done`")
	}
}

func TestRunDoneWithNoTimerStillSucceeds(t *testing.T) {
	withTempConfigHome(t)

	var stdout, stderr bytes.Buffer
	code := runDone(2, 2026, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "0s") {
		t.Errorf("stdout = %q, want it to report 0s elapsed", stdout.String())
	}
}

// TestRunViaDispatchDone exercises `crust done` through run() (main.go's
// dispatch), confirming parseDoneArgs and runDone are actually wired
// up under the "done" case.
func TestRunViaDispatchDone(t *testing.T) {
	withTempConfigHome(t)
	if err := aoc.StartTimer(2026, 3); err != nil {
		t.Fatalf("StartTimer: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"done", "3"}, nil, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Day 3, 2026") {
		t.Errorf("stdout = %q, want it to mention day 3", stdout.String())
	}
}
