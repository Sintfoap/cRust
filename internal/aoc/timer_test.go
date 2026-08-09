package aoc

import (
	"testing"
	"time"
)

func withTempTimerDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	old := timerDir
	timerDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { timerDir = old })
}

func TestElapsedWithNoRecordIsZeroAndNotRunning(t *testing.T) {
	withTempTimerDir(t)

	elapsed, running := Elapsed(2026, 1)
	if elapsed != 0 || running {
		t.Fatalf("got %v, %v, want 0, false", elapsed, running)
	}
}

func TestStartTimerThenElapsedIsRunning(t *testing.T) {
	withTempTimerDir(t)

	if err := StartTimer(2026, 1); err != nil {
		t.Fatalf("StartTimer: %v", err)
	}
	elapsed, running := Elapsed(2026, 1)
	if !running {
		t.Fatal("expected running=true")
	}
	if elapsed < 0 {
		t.Fatalf("got negative elapsed %v", elapsed)
	}
}

func TestStartTimerTwiceDoesNotResetProgress(t *testing.T) {
	withTempTimerDir(t)

	if err := StartTimer(2026, 1); err != nil {
		t.Fatalf("StartTimer: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	first, _ := Elapsed(2026, 1)

	if err := StartTimer(2026, 1); err != nil {
		t.Fatalf("StartTimer (again): %v", err)
	}
	second, running := Elapsed(2026, 1)
	if !running {
		t.Fatal("expected still running")
	}
	if second < first {
		t.Fatalf("second start reset the clock: first=%v second=%v", first, second)
	}
}

func TestStopTimerBanksElapsedAndStopsRunning(t *testing.T) {
	withTempTimerDir(t)

	if err := StartTimer(2026, 2); err != nil {
		t.Fatalf("StartTimer: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if err := StopTimer(2026, 2); err != nil {
		t.Fatalf("StopTimer: %v", err)
	}

	elapsed, running := Elapsed(2026, 2)
	if running {
		t.Fatal("expected running=false after StopTimer")
	}
	if elapsed <= 0 {
		t.Fatalf("got elapsed %v, want > 0", elapsed)
	}

	// Time should stay frozen after stopping — no more accrual.
	frozen := elapsed
	time.Sleep(5 * time.Millisecond)
	elapsed2, _ := Elapsed(2026, 2)
	if elapsed2 != frozen {
		t.Fatalf("elapsed kept accruing after stop: %v -> %v", frozen, elapsed2)
	}
}

func TestStopTimerWithNoRunningTimerIsNoOp(t *testing.T) {
	withTempTimerDir(t)

	if err := StopTimer(2026, 3); err != nil {
		t.Fatalf("StopTimer on never-started day: %v", err)
	}
	elapsed, running := Elapsed(2026, 3)
	if elapsed != 0 || running {
		t.Fatalf("got %v, %v, want 0, false", elapsed, running)
	}
}

func TestTimersForDifferentDaysAreIndependent(t *testing.T) {
	withTempTimerDir(t)

	if err := StartTimer(2026, 1); err != nil {
		t.Fatalf("StartTimer day 1: %v", err)
	}
	if err := StartTimer(2025, 1); err != nil {
		t.Fatalf("StartTimer 2025 day 1: %v", err)
	}
	if err := StopTimer(2026, 1); err != nil {
		t.Fatalf("StopTimer day 1: %v", err)
	}

	_, running2026 := Elapsed(2026, 1)
	_, running2025 := Elapsed(2025, 1)
	if running2026 {
		t.Fatal("2026 day 1 should be stopped")
	}
	if !running2025 {
		t.Fatal("2025 day 1 should still be running")
	}
}
