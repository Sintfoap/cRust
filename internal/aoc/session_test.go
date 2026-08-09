package aoc

import (
	"os"
	"path/filepath"
	"testing"
)

func withTempSessionDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := sessionDir
	sessionDir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { sessionDir = old })
	return dir
}

func TestLoadSessionMissingReturnsNotOK(t *testing.T) {
	withTempSessionDir(t)
	t.Setenv("AOC_SESSION", "")

	if _, ok := LoadSession(); ok {
		t.Fatal("expected ok=false with no saved session and no env var")
	}
}

func TestSaveThenLoadSessionRoundTrips(t *testing.T) {
	withTempSessionDir(t)
	t.Setenv("AOC_SESSION", "")

	if err := SaveSession("abc123"); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	got, ok := LoadSession()
	if !ok {
		t.Fatal("expected ok=true after SaveSession")
	}
	if got != "abc123" {
		t.Fatalf("got %q, want %q", got, "abc123")
	}
}

func TestSaveSessionTrimsWhitespaceAndWritesPrivateFile(t *testing.T) {
	dir := withTempSessionDir(t)
	t.Setenv("AOC_SESSION", "")

	if err := SaveSession("  abc123\n\n"); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	got, ok := LoadSession()
	if !ok || got != "abc123" {
		t.Fatalf("got %q, %v, want %q, true", got, ok, "abc123")
	}

	path := filepath.Join(dir, "crust", "session")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("got mode %o, want 0600", perm)
	}
}

func TestAOCSessionEnvVarOverridesSavedFile(t *testing.T) {
	withTempSessionDir(t)
	if err := SaveSession("from-file"); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	t.Setenv("AOC_SESSION", "from-env")

	got, ok := LoadSession()
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got != "from-env" {
		t.Fatalf("got %q, want %q (env var should win)", got, "from-env")
	}
}
