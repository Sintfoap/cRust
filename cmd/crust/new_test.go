package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseNewArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantDay  int
		wantForc bool
		wantErr  bool
	}{
		{"day only", []string{"7"}, 7, false, false},
		{"force flag", []string{"3", "--force"}, 3, true, false},
		{"force before day", []string{"--force", "3"}, 3, true, false},
		{"no args", nil, 0, false, true},
		{"bad day", []string{"nope"}, 0, false, true},
		{"day zero", []string{"0"}, 0, false, true},
		{"day out of range", []string{"26"}, 0, false, true},
		{"unknown flag", []string{"1", "--bogus"}, 0, false, true},
		{"two positional args", []string{"1", "2"}, 0, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			day, force, err := parseNewArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseNewArgs(%v) error = nil, want an error", tt.args)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseNewArgs(%v) error = %v, want nil", tt.args, err)
			}
			if day != tt.wantDay || force != tt.wantForc {
				t.Errorf("parseNewArgs(%v) = (%d, %v), want (%d, %v)", tt.args, day, force, tt.wantDay, tt.wantForc)
			}
		})
	}
}

func TestRunNewCreatesFileFromTemplate(t *testing.T) {
	dir := t.TempDir()
	restore := chdirTemp(t, dir)
	t.Cleanup(restore)

	var stdout, stderr bytes.Buffer
	code := runNew(7, false, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}

	data, err := os.ReadFile(filepath.Join(dir, "day07.crust"))
	if err != nil {
		t.Fatalf("reading created file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Day 07 starter") {
		t.Errorf("content missing day header, got:\n%s", content)
	}
	if !strings.Contains(content, "recipe store_part1() {") || !strings.Contains(content, "recipe store_part2() {") {
		t.Errorf("content missing store_part1/store_part2 stubs, got:\n%s", content)
	}
	if !strings.Contains(content, "day07_input.txt") {
		t.Errorf("content should reference day07_input.txt, got:\n%s", content)
	}
	if !strings.Contains(stdout.String(), "day07.crust") {
		t.Errorf("stdout = %q, want it to mention day07.crust", stdout.String())
	}
}

func TestRunNewRefusesToOverwriteWithoutForce(t *testing.T) {
	dir := t.TempDir()
	restore := chdirTemp(t, dir)
	t.Cleanup(restore)

	path := filepath.Join(dir, "day03.crust")
	if err := os.WriteFile(path, []byte("existing content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runNew(3, false, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "--force") {
		t.Errorf("stderr = %q, want it to mention --force", stderr.String())
	}

	data, _ := os.ReadFile(path)
	if string(data) != "existing content\n" {
		t.Errorf("existing file was overwritten: %q", data)
	}
}

func TestRunNewForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	restore := chdirTemp(t, dir)
	t.Cleanup(restore)

	path := filepath.Join(dir, "day03.crust")
	if err := os.WriteFile(path, []byte("existing content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runNew(3, true, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}

	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "existing content") {
		t.Errorf("file was not overwritten: %q", data)
	}
}

// TestRunNewCreatedFileParsesAndRuns confirms the stamped-out file is
// actual valid cRust, not just text that happens to contain the right
// substrings — parsed and executed for real through runFile, the same
// way examples_test.go verifies every example under examples/.
func TestRunNewCreatedFileParsesAndRuns(t *testing.T) {
	dir := t.TempDir()
	restore := chdirTemp(t, dir)
	t.Cleanup(restore)

	var stdout, stderr bytes.Buffer
	if code := runNew(1, false, &stdout, &stderr); code != 0 {
		t.Fatalf("runNew exit code = %d, want 0; stderr = %q", code, stderr.String())
	}

	path := filepath.Join(dir, "day01.crust")
	stdout.Reset()
	stderr.Reset()
	code := runFile(path, "part1", strings.NewReader("a\nb\nc\n"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runFile exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "3" {
		t.Errorf("stdout = %q, want %q (3 lines of input)", stdout.String(), "3")
	}
}

// TestRunViaDispatchNew exercises `crust new` through run() (main.go's
// dispatch), confirming parseNewArgs and runNew are actually wired up
// under the "new" case.
func TestRunViaDispatchNew(t *testing.T) {
	dir := t.TempDir()
	restore := chdirTemp(t, dir)
	t.Cleanup(restore)

	var stdout, stderr bytes.Buffer
	code := run([]string{"new", "9"}, nil, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}

	if _, err := os.Stat(filepath.Join(dir, "day09.crust")); err != nil {
		t.Errorf("expected day09.crust to exist: %v", err)
	}
}
