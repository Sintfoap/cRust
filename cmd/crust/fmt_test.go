package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFmtArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantPath  string
		wantWrite bool
		wantErr   bool
	}{
		{"file only", []string{"day01.crust"}, "day01.crust", false, false},
		{"file then -w", []string{"day01.crust", "-w"}, "day01.crust", true, false},
		{"-w then file", []string{"-w", "day01.crust"}, "day01.crust", true, false},
		{"--write long form", []string{"day01.crust", "--write"}, "day01.crust", true, false},
		{"no args", nil, "", false, true},
		{"unknown flag", []string{"day01.crust", "--bogus"}, "", false, true},
		{"two positional args", []string{"day01.crust", "day02.crust"}, "", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, write, err := parseFmtArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseFmtArgs(%v) error = nil, want an error", tt.args)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseFmtArgs(%v) error = %v, want nil", tt.args, err)
			}
			if path != tt.wantPath || write != tt.wantWrite {
				t.Errorf("parseFmtArgs(%v) = (%q, %v), want (%q, %v)", tt.args, path, write, tt.wantPath, tt.wantWrite)
			}
		})
	}
}

func writeFmtTestFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "prog.crust")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunFmtPrintsToStdoutByDefault(t *testing.T) {
	path := writeFmtTestFile(t, "x=1\ndeliver(x)\n")
	var stdout, stderr bytes.Buffer
	code := runFmt(path, false, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "x = 1\ndeliver(x)\n") {
		t.Errorf("stdout = %q, want the formatted source", stdout.String())
	}
	// -- and the file on disk must be untouched.
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(onDisk) != "x=1\ndeliver(x)\n" {
		t.Errorf("file on disk = %q, want it unchanged (no -w given)", onDisk)
	}
}

func TestRunFmtWriteFlagRewritesFile(t *testing.T) {
	path := writeFmtTestFile(t, "x=1\ndeliver(x)\n")
	var stdout, stderr bytes.Buffer
	code := runFmt(path, true, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if stdout.String() != "" {
		t.Errorf("stdout = %q, want nothing printed when writing in place", stdout.String())
	}
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "x = 1\ndeliver(x)\n"
	if string(onDisk) != want {
		t.Errorf("file on disk = %q, want %q", onDisk, want)
	}
}

func TestRunFmtWriteFlagOnAlreadyFormattedFileIsNoOp(t *testing.T) {
	// Confirms the mtime-preserving skip (runFmt's "formatted ==
	// string(src)" check) actually takes effect: an already-canonical
	// file's mtime shouldn't change on a -w run.
	path := writeFmtTestFile(t, "x = 1\ndeliver(x)\n")
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runFmt(path, true, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Errorf("mtime changed from %v to %v, want unchanged for an already-formatted file", before.ModTime(), after.ModTime())
	}
}

func TestRunFmtMissingFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runFmt("/no/such/file.crust", false, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "no such file") {
		t.Errorf("stderr = %q, want it to mention the missing file", stderr.String())
	}
}

func TestRunFmtParseErrorReportsClearly(t *testing.T) {
	path := writeFmtTestFile(t, "x = (\n")
	var stdout, stderr bytes.Buffer
	code := runFmt(path, false, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "parse error") {
		t.Errorf("stderr = %q, want it to mention a parse error", stderr.String())
	}
	if stdout.String() != "" {
		t.Errorf("stdout = %q, want nothing printed on a parse error", stdout.String())
	}
}

// TestRunViaDispatchFormatsARealFile exercises the full crust fmt
// command through run() (main.go's dispatch), not just runFmt
// directly -- confirms parseFmtArgs and runFmt are actually wired up
// under the "fmt" case.
func TestRunViaDispatchFormatsARealFile(t *testing.T) {
	path := writeFmtTestFile(t, "x=1\ndeliver(x)\n")
	var stdout, stderr bytes.Buffer
	code := run([]string{"fmt", path}, nil, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "x = 1") {
		t.Errorf("stdout = %q, want formatted output", stdout.String())
	}
}

func TestRunViaDispatchMissingFileArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"fmt"}, nil, &stdout, &stderr, false)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "missing <file.crust>") {
		t.Errorf("stderr = %q, want it to mention the missing file argument", stderr.String())
	}
}
