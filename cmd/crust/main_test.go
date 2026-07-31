package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunLSP(t *testing.T) {
	// A minimal real session: initialize then exit, framed exactly the
	// way a real client would send it — exercises `crust lsp`'s wiring
	// through run(), not just internal/lsp's own tests.
	init := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	exit := `{"jsonrpc":"2.0","method":"exit"}`
	var stdin bytes.Buffer
	for _, body := range []string{init, exit} {
		fmt.Fprintf(&stdin, "Content-Length: %d\r\n\r\n%s", len(body), body)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"lsp"}, &stdin, &stdout, &stderr, false)

	if code != 0 {
		t.Errorf("exit code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"hoverProvider":true`) {
		t.Errorf("stdout = %q, want an initialize response advertising hover support", stdout.String())
	}
}

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
			name:       "run with missing file",
			args:       []string{"run", "day01.crust"},
			wantCode:   1,
			wantStderr: "no such file",
		},
		{
			name:       "run with a real file",
			args:       []string{"run", "../../examples/hello.crust"},
			wantCode:   0,
			wantStdout: "Hello, World!",
		},
		{
			name:       "repl stub",
			args:       []string{"repl"},
			wantCode:   1,
			wantStderr: "repl: not in the oven yet",
		},
		{
			name:       "bare file shorthand",
			args:       []string{"../../examples/hello.crust"},
			wantCode:   0,
			wantStdout: "Hello, World!",
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
			name:       "parse missing file",
			args:       []string{"parse"},
			wantCode:   1,
			wantStderr: "missing <file.crust>",
		},
		{
			name:       "parse missing on disk",
			args:       []string{"parse", "/no/such/file.crust"},
			wantCode:   1,
			wantStderr: "no such file",
		},
		{
			name:       "parse with a real file",
			args:       []string{"parse", "../../examples/hello.crust"},
			wantCode:   0,
			wantStdout: "#1:",
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
			code := run(tt.args, strings.NewReader(""), &stdout, &stderr, tt.colorDefault)

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
		run([]string{"--no-color", "--help"}, strings.NewReader(""), &stdout, &stderr, true)
		if strings.Contains(stdout.String(), "\x1b[") {
			t.Errorf("expected no ANSI escapes with --no-color, got: %q", stdout.String())
		}
	})

	t.Run("no-banner actually removes the pizza", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		run([]string{"--no-banner", "--help"}, strings.NewReader(""), &stdout, &stderr, false)
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

func TestRunParse(t *testing.T) {
	t.Run("valid source prints one line per statement", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "ok.crust")
		if err := os.WriteFile(path, []byte("x = 1\ny = x + 2\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		var stdout, stderr bytes.Buffer
		code := runParse(path, &stdout, &stderr)

		if code != 0 {
			t.Errorf("exit code = %d, want 0; stderr = %q", code, stderr.String())
		}
		out := stdout.String()
		for _, want := range []string{"#1: x = 1", "#2: y = (x + 2)"} {
			if !strings.Contains(out, want) {
				t.Errorf("output missing %q, got:\n%s", want, out)
			}
		}
	})

	t.Run("malformed input still prints what parsed but fails", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "bad.crust")
		if err := os.WriteFile(path, []byte("x = 1\norder (a < b {\n serve 1\n}\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		var stdout, stderr bytes.Buffer
		code := runParse(path, &stdout, &stderr)

		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		if !strings.Contains(stdout.String(), "#1: x = 1") {
			t.Errorf("expected the statement before the error to still print, got: %q", stdout.String())
		}
		if !strings.Contains(stderr.String(), "parse error") {
			t.Errorf("expected a parse error note on stderr, got: %q", stderr.String())
		}
	})

	t.Run("missing file", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := runParse("/no/such/file.crust", &stdout, &stderr)

		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "no such file") {
			t.Errorf("stderr = %q, want a file-not-found message", stderr.String())
		}
	})
}

func TestParseRunArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantPath  string
		wantStore string
		wantErr   bool
	}{
		{"file only", []string{"day01.crust"}, "day01.crust", "", false},
		{"file then store", []string{"day01.crust", "--store=part1"}, "day01.crust", "part1", false},
		{"store then file", []string{"--store=part2", "day01.crust"}, "day01.crust", "part2", false},
		{"no args", nil, "", "", false},
		{"bare --store with no value", []string{"day01.crust", "--store"}, "", "", true},
		{"unknown flag", []string{"day01.crust", "--bogus"}, "", "", true},
		{"two positional args", []string{"day01.crust", "day02.crust"}, "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, store, err := parseRunArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseRunArgs(%v) = %q, %q, want an error", tt.args, path, store)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseRunArgs(%v) unexpected error: %v", tt.args, err)
			}
			if path != tt.wantPath || store != tt.wantStore {
				t.Errorf("parseRunArgs(%v) = %q, %q, want %q, %q", tt.args, path, store, tt.wantPath, tt.wantStore)
			}
		})
	}
}

func TestRunFile(t *testing.T) {
	writeFile := func(t *testing.T, content string) string {
		t.Helper()
		dir := t.TempDir()
		path := filepath.Join(dir, "prog.crust")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	t.Run("plain script with no entry point runs top-to-bottom", func(t *testing.T) {
		path := writeFile(t, `deliver("hi")`+"\n")
		var stdout, stderr bytes.Buffer
		code := runFile(path, "", strings.NewReader(""), &stdout, &stderr)
		if code != 0 {
			t.Errorf("exit code = %d, want 0; stderr = %q", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "hi") {
			t.Errorf("stdout = %q, want it to contain %q", stdout.String(), "hi")
		}
	})

	t.Run("bare store runs by default", func(t *testing.T) {
		path := writeFile(t, `recipe store() { deliver("default") }`+"\n")
		var stdout, stderr bytes.Buffer
		code := runFile(path, "", strings.NewReader(""), &stdout, &stderr)
		if code != 0 {
			t.Errorf("exit code = %d, want 0; stderr = %q", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "default") {
			t.Errorf("stdout = %q, want it to contain %q", stdout.String(), "default")
		}
	})

	t.Run("--store selects the named entry point", func(t *testing.T) {
		path := writeFile(t, `
recipe store_part1() { deliver("one") }
recipe store_part2() { deliver("two") }
`)
		var stdout, stderr bytes.Buffer
		code := runFile(path, "part2", strings.NewReader(""), &stdout, &stderr)
		if code != 0 {
			t.Errorf("exit code = %d, want 0; stderr = %q", code, stderr.String())
		}
		out := stdout.String()
		if strings.Contains(out, "one") || !strings.Contains(out, "two") {
			t.Errorf("stdout = %q, want only %q", out, "two")
		}
	})

	t.Run("--store with an unknown name is an error", func(t *testing.T) {
		path := writeFile(t, `recipe store_part1() { deliver("one") }`+"\n")
		var stdout, stderr bytes.Buffer
		code := runFile(path, "nope", strings.NewReader(""), &stdout, &stderr)
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), `no entry point named "nope"`) {
			t.Errorf("stderr = %q, want a no-entry-point-named message", stderr.String())
		}
	})

	t.Run("named parts but no bare store and no --store lists the options", func(t *testing.T) {
		path := writeFile(t, `
recipe store_part1() { deliver("one") }
recipe store_part2() { deliver("two") }
`)
		var stdout, stderr bytes.Buffer
		code := runFile(path, "", strings.NewReader(""), &stdout, &stderr)
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "--store=part1") || !strings.Contains(stderr.String(), "--store=part2") {
			t.Errorf("stderr = %q, want it to list both available entry points", stderr.String())
		}
	})

	t.Run("parse error is reported and nothing runs", func(t *testing.T) {
		path := writeFile(t, "order (a < b {\n serve 1\n}\n")
		var stdout, stderr bytes.Buffer
		code := runFile(path, "", strings.NewReader(""), &stdout, &stderr)
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "parse error") {
			t.Errorf("stderr = %q, want a parse error message", stderr.String())
		}
	})

	t.Run("runtime error is reported with file position", func(t *testing.T) {
		path := writeFile(t, "x = 1 / 0\n")
		var stdout, stderr bytes.Buffer
		code := runFile(path, "", strings.NewReader(""), &stdout, &stderr)
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "division by zero") {
			t.Errorf("stderr = %q, want a division-by-zero message", stderr.String())
		}
		if !strings.Contains(stderr.String(), ":1:") {
			t.Errorf("stderr = %q, want it to include the line number", stderr.String())
		}
	})

	t.Run("runtime error inside a store entry point is reported", func(t *testing.T) {
		path := writeFile(t, "recipe store() {\n deliver(1 / 0)\n}\n")
		var stdout, stderr bytes.Buffer
		code := runFile(path, "", strings.NewReader(""), &stdout, &stderr)
		if code != 1 {
			t.Errorf("exit code = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "division by zero") {
			t.Errorf("stderr = %q, want a division-by-zero message", stderr.String())
		}
	})

	t.Run("missing file", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := runFile("/no/such/file.crust", "", strings.NewReader(""), &stdout, &stderr)
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
