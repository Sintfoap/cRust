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

func TestParseFetchArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantDay  int
		wantYear int
		wantForc bool
		wantOut  string
		wantErr  bool
	}{
		{"day only", []string{"1"}, 1, defaultAoCYear, false, "day01_input.txt", false},
		{"day with year", []string{"5", "--year", "2020"}, 5, 2020, false, "day05_input.txt", false},
		{"day with year=", []string{"5", "--year=2020"}, 5, 2020, false, "day05_input.txt", false},
		{"force flag", []string{"3", "--force"}, 3, defaultAoCYear, true, "day03_input.txt", false},
		{"custom out", []string{"3", "--out", "in.txt"}, 3, defaultAoCYear, false, "in.txt", false},
		{"no args", nil, 0, 0, false, "", true},
		{"bad day", []string{"nope"}, 0, 0, false, "", true},
		{"day out of range", []string{"26"}, 0, 0, false, "", true},
		{"unknown flag", []string{"1", "--bogus"}, 0, 0, false, "", true},
		{"two positional args", []string{"1", "2"}, 0, 0, false, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			day, year, force, out, err := parseFetchArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseFetchArgs(%v) error = nil, want an error", tt.args)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseFetchArgs(%v) error = %v, want nil", tt.args, err)
			}
			if day != tt.wantDay || year != tt.wantYear || force != tt.wantForc || out != tt.wantOut {
				t.Errorf("parseFetchArgs(%v) = (%d, %d, %v, %q), want (%d, %d, %v, %q)",
					tt.args, day, year, force, out, tt.wantDay, tt.wantYear, tt.wantForc, tt.wantOut)
			}
		})
	}
}

func TestRunFetchWithNoSessionFails(t *testing.T) {
	withTempConfigHome(t)

	out := filepath.Join(t.TempDir(), "day01_input.txt")
	var stdout, stderr bytes.Buffer
	code := runFetch(1, 2026, false, out, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "crust login") {
		t.Errorf("stderr = %q, want it to mention `crust login`", stderr.String())
	}
}

func TestRunFetchDownloadsAndSavesInput(t *testing.T) {
	withTempConfigHome(t)
	if err := aoc.SaveSession("test-session"); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("1\n2\n3\n"))
	}))
	defer srv.Close()
	old := fetchBaseURL
	fetchBaseURL = srv.URL
	t.Cleanup(func() { fetchBaseURL = old })

	out := filepath.Join(t.TempDir(), "day01_input.txt")
	var stdout, stderr bytes.Buffer
	code := runFetch(1, 2026, false, out, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if string(data) != "1\n2\n3\n" {
		t.Errorf("got %q", data)
	}

	elapsed, running := aoc.Elapsed(2026, 1)
	if !running {
		t.Error("expected the timer to auto-start on a successful fetch")
	}
	if elapsed < 0 {
		t.Errorf("got negative elapsed %v", elapsed)
	}
}

func TestRunFetchRefusesToOverwriteWithoutForce(t *testing.T) {
	withTempConfigHome(t)
	if err := aoc.SaveSession("test-session"); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("new content\n"))
	}))
	defer srv.Close()
	old := fetchBaseURL
	fetchBaseURL = srv.URL
	t.Cleanup(func() { fetchBaseURL = old })

	out := filepath.Join(t.TempDir(), "day01_input.txt")
	if err := os.WriteFile(out, []byte("existing content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runFetch(1, 2026, false, out, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "--force") {
		t.Errorf("stderr = %q, want it to mention --force", stderr.String())
	}

	data, _ := os.ReadFile(out)
	if string(data) != "existing content\n" {
		t.Errorf("existing file was overwritten: %q", data)
	}
}

func TestRunFetchForceOverwrites(t *testing.T) {
	withTempConfigHome(t)
	if err := aoc.SaveSession("test-session"); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("new content\n"))
	}))
	defer srv.Close()
	old := fetchBaseURL
	fetchBaseURL = srv.URL
	t.Cleanup(func() { fetchBaseURL = old })

	out := filepath.Join(t.TempDir(), "day01_input.txt")
	if err := os.WriteFile(out, []byte("existing content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runFetch(1, 2026, true, out, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	data, _ := os.ReadFile(out)
	if string(data) != "new content\n" {
		t.Errorf("got %q, want the file overwritten", data)
	}
}

func TestRunFetchServerErrorReported(t *testing.T) {
	withTempConfigHome(t)
	if err := aoc.SaveSession("bad-session"); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad session", http.StatusBadRequest)
	}))
	defer srv.Close()
	old := fetchBaseURL
	fetchBaseURL = srv.URL
	t.Cleanup(func() { fetchBaseURL = old })

	out := filepath.Join(t.TempDir(), "day01_input.txt")
	var stdout, stderr bytes.Buffer
	code := runFetch(1, 2026, false, out, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("expected no file written on a fetch error")
	}
}

// TestRunViaDispatchFetch exercises `crust fetch` through run()
// (main.go's dispatch), confirming parseFetchArgs and runFetch are
// actually wired up under the "fetch" case.
func TestRunViaDispatchFetch(t *testing.T) {
	withTempConfigHome(t)
	if err := aoc.SaveSession("test-session"); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("dispatch content\n"))
	}))
	defer srv.Close()
	old := fetchBaseURL
	fetchBaseURL = srv.URL
	t.Cleanup(func() { fetchBaseURL = old })

	dir := t.TempDir()
	old2 := chdirTemp(t, dir)
	t.Cleanup(old2)

	var stdout, stderr bytes.Buffer
	code := run([]string{"fetch", "7"}, nil, &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}

	data, err := os.ReadFile(filepath.Join(dir, "day07_input.txt"))
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if string(data) != "dispatch content\n" {
		t.Errorf("got %q", data)
	}
}

// chdirTemp changes the working directory to dir and returns a func
// that restores it, for tests exercising commands (like `crust
// fetch`) whose default output path is relative to the cwd.
func chdirTemp(t *testing.T, dir string) func() {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() { os.Chdir(old) }
}
