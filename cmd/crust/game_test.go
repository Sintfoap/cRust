package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a bytes.Buffer safe for one writer (runGame, on its own
// goroutine) and one concurrent reader (the test, polling for the
// printed URL) -- a plain bytes.Buffer isn't safe for that, and
// runGame's own signature takes an io.Writer, not a listener, so
// there's no other way to learn which port :0 actually bound to.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

var boundURLRe = regexp.MustCompile(`http://localhost:(\d+)/`)

func TestParseGameArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantPath string
		wantPort int
		wantErr  bool
	}{
		{"no args uses default port, no file", nil, "", defaultGamePort, false},
		{"a bare file argument", []string{"day01.crust"}, "day01.crust", defaultGamePort, false},
		{"-p space form", []string{"-p", "8080"}, "", 8080, false},
		{"--port space form", []string{"--port", "8080"}, "", 8080, false},
		{"-p equals form", []string{"-p=8080"}, "", 8080, false},
		{"--port equals form", []string{"--port=8080"}, "", 8080, false},
		{"file and port together", []string{"day01.crust", "-p", "9090"}, "day01.crust", 9090, false},
		{"-p missing value", []string{"-p"}, "", 0, true},
		{"bad port not a number", []string{"-p", "nope"}, "", 0, true},
		{"bad port zero", []string{"-p", "0"}, "", 0, true},
		{"bad port too big", []string{"-p", "70000"}, "", 0, true},
		{"unknown flag", []string{"--bogus"}, "", 0, true},
		{"two bare arguments is an error", []string{"day01.crust", "day02.crust"}, "", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, port, err := parseGameArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseGameArgs(%v) error = nil, want an error", tt.args)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseGameArgs(%v) error = %v, want nil", tt.args, err)
			}
			if path != tt.wantPath || port != tt.wantPort {
				t.Errorf("parseGameArgs(%v) = (%q, %d), want (%q, %d)", tt.args, path, port, tt.wantPath, tt.wantPort)
			}
		})
	}
}

// TestServeGameServesRealContent binds an OS-assigned ephemeral port,
// starts serving in the background, and fetches index.html,
// wasm_exec.js, and crust-game.wasm over real HTTP -- confirming the
// embed is wired up and build-wasm-game.sh's assets are actually
// served at the site root.
func TestServeGameServesRealContent(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() { done <- serveGame(ln, "", "", &stdout, &stderr) }()

	base := fmt.Sprintf("http://localhost:%d/", port)

	var resp *http.Response
	for i := 0; i < 100; i++ {
		resp, err = http.Get(base)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("GET %s: %v", base, err)
	}
	body, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		t.Fatalf("reading response body: %v", readErr)
	}
	if !strings.Contains(string(body), "crustGameInit") {
		t.Errorf("index.html missing expected content: %q", truncate(string(body), 200))
	}

	for _, path := range []string{"wasm_exec.js", "crust-game.wasm"} {
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s: %v", base+path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", base+path, resp.StatusCode)
		}
	}

	ln.Close()
	code := <-done
	if code != 1 {
		t.Errorf("serveGame() after Close() = %d, want 1 (http.Serve reports the closed listener as an error)", code)
	}
	if !strings.Contains(stdout.String(), "Fresh out of the oven at "+base) {
		t.Errorf("stdout = %q, want it to mention the real bound URL %q", stdout.String(), base)
	}
	if !strings.Contains(stderr.String(), "server error") {
		t.Errorf("stderr = %q, want a server error mentioned after Close()", stderr.String())
	}
}

// TestServeGameSourceJSONReflectsGivenSource confirms /source.json --
// the endpoint index.html's own JS fetches to preload the editor --
// actually carries whatever source string serveGame was started with,
// and comes back empty (not missing/erroring) when there wasn't one.
func TestServeGameSourceJSONReflectsGivenSource(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{"no source", ""},
		{"real source", "recipe store() {\n    deliver(\"hi\")\n}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer ln.Close()
			port := ln.Addr().(*net.TCPAddr).Port
			go func() { serveGame(ln, tt.source, "", io.Discard, io.Discard) }()

			base := fmt.Sprintf("http://localhost:%d/source.json", port)
			var resp *http.Response
			for i := 0; i < 100; i++ {
				resp, err = http.Get(base)
				if err == nil {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if err != nil {
				t.Fatalf("GET %s: %v", base, err)
			}
			defer resp.Body.Close()
			var got struct{ Source string }
			if decErr := json.NewDecoder(resp.Body).Decode(&got); decErr != nil {
				t.Fatalf("decoding /source.json: %v", decErr)
			}
			if got.Source != tt.source {
				t.Errorf("/source.json source = %q, want %q", got.Source, tt.source)
			}
		})
	}
}

// TestGameHandlerServesAssetDirFallback confirms a file sitting next
// to the .crust file (an image for sprite(), an audio file for
// sound()) is actually reachable over HTTP -- the whole point of
// assetDir existing at all.
func TestGameHandlerServesAssetDirFallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cat.png"), []byte("not a real png, just bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	go func() { serveGame(ln, "", dir, io.Discard, io.Discard) }()

	base := fmt.Sprintf("http://localhost:%d/cat.png", port)
	var resp *http.Response
	for i := 0; i < 100; i++ {
		resp, err = http.Get(base)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("GET %s: %v", base, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200", base, resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "not a real png, just bytes" {
		t.Errorf("body = %q, want the local file's own content", body)
	}
}

// TestGameHandlerEmbeddedAssetsTakePriorityOverAssetDir confirms a
// same-named local file can never shadow one of the tool's own
// embedded assets -- accidentally naming a local file "wasm_exec.js"
// shouldn't be able to break the page.
func TestGameHandlerEmbeddedAssetsTakePriorityOverAssetDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "wasm_exec.js"), []byte("definitely not the real one"), 0o644); err != nil {
		t.Fatal(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	go func() { serveGame(ln, "", dir, io.Discard, io.Discard) }()

	base := fmt.Sprintf("http://localhost:%d/wasm_exec.js", port)
	var resp *http.Response
	for i := 0; i < 100; i++ {
		resp, err = http.Get(base)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("GET %s: %v", base, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) == "definitely not the real one" {
		t.Error("a local file named wasm_exec.js shadowed the tool's own embedded one")
	}
}

func TestRunGameMissingFileReportsError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runGame(filepath.Join(t.TempDir(), "nope.crust"), 0, &stdout, &stderr)
	if code != 1 {
		t.Errorf("runGame() on a missing file = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "no such file") && !strings.Contains(stderr.String(), "cannot find") {
		t.Errorf("stderr = %q, want it to mention the missing file", stderr.String())
	}
}

func TestRunGameListenFailureReportsError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	var stdout, stderr bytes.Buffer
	code := runGame("", port, &stdout, &stderr)
	if code != 1 {
		t.Errorf("runGame() on an already-bound port = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "cannot serve on port") {
		t.Errorf("stderr = %q, want it to mention the bind failure", stderr.String())
	}
}

func TestRunGameSuccessPathBindsAndServes(t *testing.T) {
	done := make(chan int, 1)
	go func() { done <- runGame("", 0, io.Discard, io.Discard) }()
	select {
	case code := <-done:
		t.Fatalf("runGame(\"\", 0, ...) returned %d immediately, want it to block serving", code)
	case <-time.After(100 * time.Millisecond):
	}
}

// TestRunGameWithFilePreloadsSource confirms runGame actually reads
// the given path and hands its content through to serveGame's source
// (which TestServeGameSourceJSONReflectsGivenSource already proves
// /source.json exposes correctly) -- an end-to-end check through the
// real subcommand rather than just its two halves in isolation. Uses
// port 0 (OS-assigned) and recovers the real bound port from runGame's
// own stdout banner, rather than pre-binding and closing an ephemeral
// port and hoping nothing else grabs it before runGame does.
func TestRunGameWithFilePreloadsSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "day01.crust")
	const src = "recipe store() {\n    deliver(\"preloaded\")\n}\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout syncBuffer
	done := make(chan int, 1)
	go func() { done <- runGame(path, 0, &stdout, io.Discard) }()

	var base string
	for i := 0; i < 200; i++ {
		if m := boundURLRe.FindStringSubmatch(stdout.String()); m != nil {
			base = m[0] + "source.json"
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if base == "" {
		t.Fatalf("runGame never printed a bound URL; stdout = %q", stdout.String())
	}

	var resp *http.Response
	var err error
	for i := 0; i < 100; i++ {
		resp, err = http.Get(base)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("GET %s: %v", base, err)
	}
	defer resp.Body.Close()
	var got struct{ Source string }
	if decErr := json.NewDecoder(resp.Body).Decode(&got); decErr != nil {
		t.Fatalf("decoding /source.json: %v", decErr)
	}
	if got.Source != src {
		t.Errorf("/source.json source = %q, want the preloaded file content %q", got.Source, src)
	}
}

func TestRunGameSubcommandBadPort(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"game", "--port", "not-a-number"}, nil, &stdout, &stderr, false)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "invalid port") {
		t.Errorf("stderr = %q, want it to mention the invalid port", stderr.String())
	}
}

func TestRunGameSubcommandTwoFilesIsError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"game", "a.crust", "b.crust"}, nil, &stdout, &stderr, false)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "at most one file") {
		t.Errorf("stderr = %q, want it to mention the one-file limit", stderr.String())
	}
}
