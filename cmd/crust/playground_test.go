package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestParsePlaygroundArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantPort int
		wantErr  bool
	}{
		{"no args uses default", nil, defaultPlaygroundPort, false},
		{"-p space form", []string{"-p", "8080"}, 8080, false},
		{"--port space form", []string{"--port", "8080"}, 8080, false},
		{"-p equals form", []string{"-p=8080"}, 8080, false},
		{"--port equals form", []string{"--port=8080"}, 8080, false},
		{"-p missing value", []string{"-p"}, 0, true},
		{"bad port not a number", []string{"-p", "nope"}, 0, true},
		{"bad port zero", []string{"-p", "0"}, 0, true},
		{"bad port too big", []string{"-p", "70000"}, 0, true},
		{"unknown flag", []string{"--bogus"}, 0, true},
		{"bare argument is an error", []string{"day01.crust"}, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port, err := parsePlaygroundArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parsePlaygroundArgs(%v) error = nil, want an error", tt.args)
				}
				return
			}
			if err != nil {
				t.Fatalf("parsePlaygroundArgs(%v) error = %v, want nil", tt.args, err)
			}
			if port != tt.wantPort {
				t.Errorf("parsePlaygroundArgs(%v) = %d, want %d", tt.args, port, tt.wantPort)
			}
		})
	}
}

// TestServePlaygroundServesRealContent binds an OS-assigned ephemeral
// port, starts serving in the background, and fetches index.html,
// wasm_exec.js, and crust.wasm over real HTTP -- confirming the embed
// is wired up and the assets scripts/build-wasm.sh produces are
// actually served at the site root (not nested under
// /playground_assets/).
func TestServePlaygroundServesRealContent(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() { done <- servePlayground(ln, &stdout, &stderr) }()

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
	if !strings.Contains(string(body), "crustRun") {
		t.Errorf("index.html missing expected content: %q", truncate(string(body), 200))
	}

	for _, path := range []string{"wasm_exec.js", "crust.wasm"} {
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
		t.Errorf("servePlayground() after Close() = %d, want 1 (http.Serve reports the closed listener as an error)", code)
	}
	if !strings.Contains(stdout.String(), "Fresh out of the oven at "+base) {
		t.Errorf("stdout = %q, want it to mention the real bound URL %q", stdout.String(), base)
	}
	if !strings.Contains(stderr.String(), "server error") {
		t.Errorf("stderr = %q, want a server error mentioned after Close()", stderr.String())
	}
}

func TestRunBakePlaygroundListenFailureReportsError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	var stdout, stderr bytes.Buffer
	code := runBakePlayground(port, &stdout, &stderr)
	if code != 1 {
		t.Errorf("runBakePlayground() on an already-bound port = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "cannot serve on port") {
		t.Errorf("stderr = %q, want it to mention the bind failure", stderr.String())
	}
}

func TestRunBakePlaygroundSuccessPathBindsAndServes(t *testing.T) {
	done := make(chan int, 1)
	go func() { done <- runBakePlayground(0, io.Discard, io.Discard) }()
	select {
	case code := <-done:
		t.Fatalf("runBakePlayground(0, ...) returned %d immediately, want it to block serving", code)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestRunBakePlaygroundSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"bake", "playground", "--port", "not-a-number"}, nil, &stdout, &stderr, false)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "invalid port") {
		t.Errorf("stderr = %q, want it to mention the invalid port", stderr.String())
	}
}

func TestRunBakeUnknownSubcommandMentionsBoth(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"bake", "bogus"}, nil, &stdout, &stderr, false)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "documentation") || !strings.Contains(stderr.String(), "playground") {
		t.Errorf("stderr = %q, want it to mention both documentation and playground", stderr.String())
	}
}
