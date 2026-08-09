package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Sintfoap/cRust/internal/aoc"
)

// withTempConfigHome redirects os.UserConfigDir() (and therefore
// internal/aoc's session/timer storage, which has no test-only seam
// of its own to override directly across a package boundary) at a
// fresh temp directory for the duration of one test, via
// $XDG_CONFIG_HOME — the same env var Go's own os.UserConfigDir
// already consults on this platform.
func withTempConfigHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AOC_SESSION", "")
	return dir
}

func TestRunLoginSavesSessionFromStdin(t *testing.T) {
	withTempConfigHome(t)

	stdin := strings.NewReader("my-cookie-value\n")
	var stdout, stderr bytes.Buffer
	code := runLogin(stdin, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}

	got, ok := aoc.LoadSession()
	if !ok {
		t.Fatal("expected LoadSession to find the saved cookie")
	}
	if got != "my-cookie-value" {
		t.Errorf("got %q, want %q", got, "my-cookie-value")
	}
}

func TestRunLoginTrimsWhitespace(t *testing.T) {
	withTempConfigHome(t)

	stdin := strings.NewReader("  padded-cookie  \n")
	var stdout, stderr bytes.Buffer
	if code := runLogin(stdin, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	got, _ := aoc.LoadSession()
	if got != "padded-cookie" {
		t.Errorf("got %q, want %q", got, "padded-cookie")
	}
}

func TestRunLoginEmptyInputFails(t *testing.T) {
	withTempConfigHome(t)

	stdin := strings.NewReader("\n")
	var stdout, stderr bytes.Buffer
	code := runLogin(stdin, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "empty") {
		t.Errorf("stderr = %q, want it to mention the empty cookie", stderr.String())
	}
}

func TestRunLoginNoInputFails(t *testing.T) {
	withTempConfigHome(t)

	stdin := strings.NewReader("")
	var stdout, stderr bytes.Buffer
	code := runLogin(stdin, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunLoginWarnsAboutNoMasking(t *testing.T) {
	withTempConfigHome(t)

	stdin := strings.NewReader("abc\n")
	var stdout, stderr bytes.Buffer
	if code := runLogin(stdin, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "no input masking") {
		t.Errorf("stdout = %q, want a visibility warning", stdout.String())
	}
}
