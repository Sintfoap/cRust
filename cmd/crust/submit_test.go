package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sintfoap/cRust/internal/aoc"
)

func TestParseSubmitArgs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantDay    int
		wantPart   int
		wantYear   int
		wantAnswer string
		wantErr    bool
	}{
		{"day answer part", []string{"3", "42", "--part=1"}, 3, 1, defaultAoCYear, "42", false},
		{"day part no answer", []string{"3", "--part=2"}, 3, 2, defaultAoCYear, "", false},
		{"part with space", []string{"3", "42", "--part", "1"}, 3, 1, defaultAoCYear, "42", false},
		{"with year", []string{"3", "42", "--part=1", "--year", "2020"}, 3, 1, 2020, "42", false},
		{"flags before positionals", []string{"--part=1", "3", "42"}, 3, 1, defaultAoCYear, "42", false},
		{"no args", nil, 0, 0, 0, "", true},
		{"missing part", []string{"3", "42"}, 0, 0, 0, "", true},
		{"invalid part", []string{"3", "42", "--part=3"}, 0, 0, 0, "", true},
		{"bad day", []string{"nope", "42", "--part=1"}, 0, 0, 0, "", true},
		{"day out of range", []string{"26", "42", "--part=1"}, 0, 0, 0, "", true},
		{"unknown flag", []string{"3", "42", "--part=1", "--bogus"}, 0, 0, 0, "", true},
		{"three positional args", []string{"3", "42", "43", "--part=1"}, 0, 0, 0, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			day, part, year, answer, err := parseSubmitArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseSubmitArgs(%v) error = nil, want an error", tt.args)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSubmitArgs(%v) error = %v, want nil", tt.args, err)
			}
			if day != tt.wantDay || part != tt.wantPart || year != tt.wantYear || answer != tt.wantAnswer {
				t.Errorf("parseSubmitArgs(%v) = (%d, %d, %d, %q), want (%d, %d, %d, %q)",
					tt.args, day, part, year, answer, tt.wantDay, tt.wantPart, tt.wantYear, tt.wantAnswer)
			}
		})
	}
}

func TestRunSubmitWithNoSessionFails(t *testing.T) {
	withTempConfigHome(t)

	var stdout, stderr bytes.Buffer
	code := runSubmit(1, 1, 2026, "42", strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "crust login") {
		t.Errorf("stderr = %q, want it to mention `crust login`", stderr.String())
	}
}

func TestRunSubmitCorrectAnswerExitsZero(t *testing.T) {
	withTempConfigHome(t)
	if err := aoc.SaveSession("test-session"); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<article><p>That's the right answer! <a href="/2026/day/1">[Return to Day 1]</a></p></article>`))
	}))
	defer srv.Close()
	old := submitBaseURL
	submitBaseURL = srv.URL
	t.Cleanup(func() { submitBaseURL = old })

	var stdout, stderr bytes.Buffer
	code := runSubmit(1, 1, 2026, "42", strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Correct") {
		t.Errorf("stdout = %q, want it to say Correct", stdout.String())
	}
}

func TestRunSubmitWrongAnswerExitsOne(t *testing.T) {
	withTempConfigHome(t)
	if err := aoc.SaveSession("test-session"); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<article><p>That's not the right answer; your answer is too low. <a href="/2026/day/1">[Return to Day 1]</a></p></article>`))
	}))
	defer srv.Close()
	old := submitBaseURL
	submitBaseURL = srv.URL
	t.Cleanup(func() { submitBaseURL = old })

	var stdout, stderr bytes.Buffer
	code := runSubmit(1, 1, 2026, "1", strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "too low") {
		t.Errorf("stdout = %q, want it to mention 'too low'", stdout.String())
	}
}

func TestRunSubmitReadsAnswerFromStdinWhenOmitted(t *testing.T) {
	withTempConfigHome(t)
	if err := aoc.SaveSession("test-session"); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	var gotAnswer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		gotAnswer = r.FormValue("answer")
		w.Write([]byte(`<article><p>That's the right answer!</p></article>`))
	}))
	defer srv.Close()
	old := submitBaseURL
	submitBaseURL = srv.URL
	t.Cleanup(func() { submitBaseURL = old })

	var stdout, stderr bytes.Buffer
	code := runSubmit(1, 2, 2026, "", strings.NewReader("piped-answer\n"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if gotAnswer != "piped-answer" {
		t.Errorf("submitted answer = %q, want %q", gotAnswer, "piped-answer")
	}
}

func TestRunSubmitEmptyStdinAndNoAnswerFails(t *testing.T) {
	withTempConfigHome(t)

	var stdout, stderr bytes.Buffer
	code := runSubmit(1, 1, 2026, "", strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "no answer") {
		t.Errorf("stderr = %q, want it to mention no answer given", stderr.String())
	}
}

// TestRunViaDispatchSubmit exercises `crust submit` through run()
// (main.go's dispatch), confirming parseSubmitArgs and runSubmit are
// actually wired up under the "submit" case.
func TestRunViaDispatchSubmit(t *testing.T) {
	withTempConfigHome(t)
	if err := aoc.SaveSession("test-session"); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<article><p>That's the right answer!</p></article>`))
	}))
	defer srv.Close()
	old := submitBaseURL
	submitBaseURL = srv.URL
	t.Cleanup(func() { submitBaseURL = old })

	var stdout, stderr bytes.Buffer
	code := run([]string{"submit", "1", "42", "--part=1"}, strings.NewReader(""), &stdout, &stderr, false)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Correct") {
		t.Errorf("stdout = %q, want it to say Correct", stdout.String())
	}
}
