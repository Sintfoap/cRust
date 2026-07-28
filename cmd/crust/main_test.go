package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		colorDefault bool
		wantCode     int
		wantStdout   string
		wantStderr   string
	}{
		{
			name:       "version",
			args:       []string{"--version"},
			wantCode:   0,
			wantStdout: "crust " + version,
		},
		{
			name:       "help long flag",
			args:       []string{"--help"},
			wantCode:   0,
			wantStdout: "Usage:",
		},
		{
			name:       "help short flag",
			args:       []string{"-h"},
			wantCode:   0,
			wantStdout: "Usage:",
		},
		{
			name:       "no args prints usage and fails",
			args:       []string{},
			wantCode:   1,
			wantStderr: "Usage:",
		},
		{
			name:       "run missing file",
			args:       []string{"run"},
			wantCode:   1,
			wantStderr: "missing <file.crust>",
		},
		{
			name:       "run with file",
			args:       []string{"run", "day01.crust"},
			wantCode:   1,
			wantStderr: "run day01.crust: not in the oven yet",
		},
		{
			name:       "repl stub",
			args:       []string{"repl"},
			wantCode:   1,
			wantStderr: "repl: not in the oven yet",
		},
		{
			name:       "bare file shorthand",
			args:       []string{"day01.crust"},
			wantCode:   1,
			wantStderr: "run day01.crust: not in the oven yet",
		},
		{
			name:       "tokens missing file",
			args:       []string{"tokens"},
			wantCode:   1,
			wantStderr: "missing <file.crust>",
		},
		{
			name:       "tokens missing on disk",
			args:       []string{"tokens", "/no/such/file.crust"},
			wantCode:   1,
			wantStderr: "no such file",
		},
		{
			name:       "tokens with a real file",
			args:       []string{"tokens", "../../examples/hello.crust"},
			wantCode:   0,
			wantStdout: "IDENT",
		},
		{
			name:       "help shows banner by default",
			args:       []string{"--help"},
			wantCode:   0,
			wantStdout: "@@@@@#########@@@@@",
		},
		{
			name:       "no-banner skips the pizza",
			args:       []string{"--no-banner", "--help"},
			wantCode:   0,
			wantStdout: "Usage:",
		},
		{
			name:       "unknown topping",
			args:       []string{"--toppings=anchovy", "--help"},
			wantCode:   2,
			wantStderr: `unknown topping "anchovy"`,
		},
		{
			name:         "color applied when tty-like and not suppressed",
			args:         []string{"--help"},
			colorDefault: true,
			wantCode:     0,
			wantStdout:   "\x1b[",
		},
		{
			name:         "no-color flag overrides colorDefault",
			args:         []string{"--no-color", "--help"},
			colorDefault: true,
			wantCode:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr, tt.colorDefault)

			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
			if tt.wantStdout != "" && !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout = %q, want substring %q", stdout.String(), tt.wantStdout)
			}
			if tt.wantStderr != "" && !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr = %q, want substring %q", stderr.String(), tt.wantStderr)
			}
		})
	}

	t.Run("no-color flag actually suppresses escapes", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		run([]string{"--no-color", "--help"}, &stdout, &stderr, true)
		if strings.Contains(stdout.String(), "\x1b[") {
			t.Errorf("expected no ANSI escapes with --no-color, got: %q", stdout.String())
		}
	})

	t.Run("no-banner actually removes the pizza", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		run([]string{"--no-banner", "--help"}, &stdout, &stderr, false)
		if strings.Contains(stdout.String(), "@@@@@#########@@@@@") {
			t.Errorf("expected no pizza art with --no-banner, got: %q", stdout.String())
		}
	})
}

func TestRunTokens(t *testing.T) {
	t.Run("valid source prints one line per token", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "ok.crust")
		if err := os.WriteFile(path, []byte(`x = 1`), 0o644); err != nil {
			t.Fatal(err)
		}

		var stdout, stderr bytes.Buffer
		code := runTokens(path, &stdout, &stderr)

		if code != 0 {
			t.Errorf("exit code = %d, want 0; stderr = %q", code, stderr.String())
		}
		out := stdout.String()
		for _, want := range []string{"IDENT", "x", "=", "INT", "1", "EOF"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q, got:\n%s", want, out)
			}
		}
	})

	t.Run("illegal input still prints tokens but fails", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bad.crust")
		if err := os.WriteFile(path, []byte(`x = 1 ! y`), 0o644); err != nil {
			t.Fatal(err)
		}

		var stdout, stderr bytes.Buffer
		code := runTokens(path, &stdout, &stderr)

		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		if !strings.Contains(stdout.String(), "ILLEGAL") {
			t.Errorf("expected ILLEGAL in stdout, got: %q", stdout.String())
		}
		if !strings.Contains(stderr.String(), "ILLEGAL") {
			t.Errorf("expected a note about the ILLEGAL token on stderr, got: %q", stderr.String())
		}
	})

	t.Run("missing file", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := runTokens("/no/such/file.crust", &stdout, &stderr)

		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "no such file") {
			t.Errorf("stderr = %q, want a file-not-found message", stderr.String())
		}
	})
}

func TestParseToppings(t *testing.T) {
	tests := []struct {
		spec    string
		want    map[string]bool
		wantErr bool
	}{
		{spec: "pepperoni,basil", want: map[string]bool{"pepperoni": true, "basil": true}},
		{spec: "all", want: map[string]bool{"pepperoni": true, "basil": true}},
		{spec: "plain", want: map[string]bool{}},
		{spec: "none", want: map[string]bool{}},
		{spec: "pepperoni", want: map[string]bool{"pepperoni": true}},
		{spec: "basil", want: map[string]bool{"basil": true}},
		{spec: "anchovy", wantErr: true},
		{spec: "pepperoni,anchovy", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			got, err := parseToppings(tt.spec)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseToppings(%q) = %v, want error", tt.spec, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseToppings(%q) unexpected error: %v", tt.spec, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("parseToppings(%q) = %v, want %v", tt.spec, got, tt.want)
			}
			for k := range tt.want {
				if !got[k] {
					t.Errorf("parseToppings(%q) missing topping %q", tt.spec, k)
				}
			}
		})
	}
}

func TestRenderPizzaToppingToggle(t *testing.T) {
	all := renderPizza(map[string]bool{toppingPepperoni: true, toppingBasil: true}, false)
	if !strings.Contains(all, "o") {
		t.Error("expected pepperoni 'o' characters when pepperoni is enabled")
	}
	if !strings.Contains(all, "*") {
		t.Error("expected basil '*' characters when basil is enabled")
	}

	plain := renderPizza(map[string]bool{}, false)
	if strings.Contains(plain, "o") {
		t.Error("expected no 'o' characters when pepperoni is disabled")
	}
	if strings.Contains(plain, "*") {
		t.Error("expected no '*' characters when basil is disabled")
	}
	// Shape (crust/cheese layout) must be unchanged in length.
	if len(all) != len(plain) {
		t.Errorf("disabling toppings changed the pizza's shape: %d vs %d bytes", len(all), len(plain))
	}
}
