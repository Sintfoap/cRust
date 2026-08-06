package main

import (
	"os"
	"path/filepath"
	"testing"
)

// withTempDevelStateDir points develStateDir at a fresh temp directory
// for the duration of one test, and restores the real os.UserConfigDir
// afterward — every test in this file must call this first, so none of
// them ever touch the real machine's config directory.
func withTempDevelStateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	original := develStateDir
	develStateDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { develStateDir = original })
	return dir
}

func TestLoadDevelStateMissingFileReturnsEmptyMap(t *testing.T) {
	withTempDevelStateDir(t)
	got := loadDevelState()
	if len(got) != 0 {
		t.Errorf("loadDevelState() = %v, want an empty map", got)
	}
}

func TestLoadDevelStateCorruptedFileReturnsEmptyMap(t *testing.T) {
	dir := withTempDevelStateDir(t)
	path, err := develStatePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := loadDevelState()
	if len(got) != 0 {
		t.Errorf("loadDevelState() = %v, want an empty map for a corrupted file", got)
	}
	_ = dir
}

func TestSaveThenLoadDevelStateRoundTrips(t *testing.T) {
	withTempDevelStateDir(t)
	if err := saveDevelState("/abs/day07.crust", develState{Store: "part2", Input: "/abs/input.txt"}); err != nil {
		t.Fatalf("saveDevelState: %v", err)
	}
	got := loadDevelState()
	want := develState{Store: "part2", Input: "/abs/input.txt"}
	if got["/abs/day07.crust"] != want {
		t.Errorf("loadDevelState()[...] = %+v, want %+v", got["/abs/day07.crust"], want)
	}
}

func TestSaveDevelStatePreservesOtherEntries(t *testing.T) {
	withTempDevelStateDir(t)
	if err := saveDevelState("/abs/day01.crust", develState{Store: "part1"}); err != nil {
		t.Fatalf("saveDevelState: %v", err)
	}
	if err := saveDevelState("/abs/day02.crust", develState{Store: "part2"}); err != nil {
		t.Fatalf("saveDevelState: %v", err)
	}
	got := loadDevelState()
	if len(got) != 2 {
		t.Fatalf("loadDevelState() = %v, want 2 entries", got)
	}
	if got["/abs/day01.crust"].Store != "part1" {
		t.Errorf("day01 store = %q, want %q", got["/abs/day01.crust"].Store, "part1")
	}
	if got["/abs/day02.crust"].Store != "part2" {
		t.Errorf("day02 store = %q, want %q", got["/abs/day02.crust"].Store, "part2")
	}
}

func TestSaveDevelStateOverwritesSameKey(t *testing.T) {
	withTempDevelStateDir(t)
	saveDevelState("/abs/day01.crust", develState{Store: "part1", Input: "first.txt"})
	saveDevelState("/abs/day01.crust", develState{Store: "part2", Input: "second.txt"})
	got := loadDevelState()
	if len(got) != 1 {
		t.Fatalf("loadDevelState() = %v, want 1 entry (overwritten, not appended)", got)
	}
	want := develState{Store: "part2", Input: "second.txt"}
	if got["/abs/day01.crust"] != want {
		t.Errorf("got %+v, want %+v", got["/abs/day01.crust"], want)
	}
}

func TestDevelStatePathErrorPropagates(t *testing.T) {
	original := develStateDir
	develStateDir = func() (string, error) { return "", os.ErrPermission }
	defer func() { develStateDir = original }()

	if _, err := develStatePath(); err == nil {
		t.Error("develStatePath() = nil error, want the underlying develStateDir error")
	}
	if got := loadDevelState(); len(got) != 0 {
		t.Errorf("loadDevelState() = %v, want an empty map when develStatePath fails", got)
	}
	if err := saveDevelState("/abs/x.crust", develState{}); err == nil {
		t.Error("saveDevelState() = nil error, want the underlying develStateDir error")
	}
}

func TestApplySavedStoreExplicitFlagWins(t *testing.T) {
	dir := withTempDevelStateDir(t)
	path := filepath.Join(dir, "day07.crust")
	saveDevelState(mustAbs(t, path), develState{Store: "part2"})

	got := applySavedStore(path, debugOptions{Store: "part1"})
	if got.Store != "part1" {
		t.Errorf("Store = %q, want %q (explicit flag should win)", got.Store, "part1")
	}
}

func TestApplySavedStoreFillsInWhenUnset(t *testing.T) {
	dir := withTempDevelStateDir(t)
	path := filepath.Join(dir, "day07.crust")
	saveDevelState(mustAbs(t, path), develState{Store: "part2"})

	got := applySavedStore(path, debugOptions{})
	if got.Store != "part2" {
		t.Errorf("Store = %q, want %q (remembered store)", got.Store, "part2")
	}
}

func TestApplySavedStoreNoSavedEntryLeavesEmpty(t *testing.T) {
	withTempDevelStateDir(t)
	got := applySavedStore("/no/such/day07.crust", debugOptions{})
	if got.Store != "" {
		t.Errorf("Store = %q, want empty (nothing remembered)", got.Store)
	}
}

func TestRestoreRunInputFillsFromSavedState(t *testing.T) {
	dir := withTempDevelStateDir(t)
	path := filepath.Join(dir, "day07.crust")
	saveDevelState(mustAbs(t, path), develState{Store: "part1", Input: "puzzle.txt"})

	got := restoreRunInput(path)
	if got.String() != "puzzle.txt" {
		t.Errorf("restoreRunInput().String() = %q, want %q", got.String(), "puzzle.txt")
	}
	if got.cursor != len([]rune("puzzle.txt")) {
		t.Errorf("cursor = %d, want cursor at end of the restored text", got.cursor)
	}
}

func TestRestoreRunInputNoSavedEntryIsZeroValue(t *testing.T) {
	withTempDevelStateDir(t)
	got := restoreRunInput("/no/such/day07.crust")
	if got.String() != "" {
		t.Errorf("restoreRunInput().String() = %q, want empty", got.String())
	}
}

func TestRestoreRunInputEmptySavedInputIsZeroValue(t *testing.T) {
	dir := withTempDevelStateDir(t)
	path := filepath.Join(dir, "day07.crust")
	saveDevelState(mustAbs(t, path), develState{Store: "part1", Input: ""})

	got := restoreRunInput(path)
	if got.String() != "" {
		t.Errorf("restoreRunInput().String() = %q, want empty when no input was ever saved", got.String())
	}
}

func TestSaveDevelStateBestEffortPersistsSelection(t *testing.T) {
	dir := withTempDevelStateDir(t)
	path := filepath.Join(dir, "day07.crust")

	saveDevelStateBestEffort(path, "part2", "puzzle.txt")

	got := loadDevelState()[mustAbs(t, path)]
	want := develState{Store: "part2", Input: "puzzle.txt"}
	if got != want {
		t.Errorf("saved state = %+v, want %+v", got, want)
	}
}

// TestSaveDevelStateMkdirFailureReturnsError points develStateDir at
// a plain file instead of a directory, so the "crust" subdirectory
// saveDevelState needs can never be created (ENOTDIR) -- a portable,
// root-safe way to trigger a filesystem failure, unlike a
// permission-bit trick that root simply bypasses.
func TestSaveDevelStateMkdirFailureReturnsError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	original := develStateDir
	develStateDir = func() (string, error) { return blocker, nil }
	defer func() { develStateDir = original }()

	if err := saveDevelState("/abs/x.crust", develState{}); err == nil {
		t.Error("saveDevelState() = nil error, want an error when the config directory can't be created")
	}
}

// TestSaveDevelStateWriteFailureReturnsError pre-creates a directory
// at the exact path saveDevelState's own temp file needs to occupy,
// so os.WriteFile fails (EISDIR) -- same root-safety reasoning as
// TestSaveDevelStateMkdirFailureReturnsError above.
func TestSaveDevelStateWriteFailureReturnsError(t *testing.T) {
	withTempDevelStateDir(t)
	path, err := develStatePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(path+".tmp", 0o755); err != nil {
		t.Fatal(err)
	}

	if err := saveDevelState("/abs/x.crust", develState{}); err == nil {
		t.Error("saveDevelState() = nil error, want an error when the temp file can't be written")
	}
}

func mustAbs(t *testing.T, path string) string {
	t.Helper()
	abs, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}
