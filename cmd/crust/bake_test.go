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

func TestParseBakeArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantPort int
		wantErr  bool
	}{
		{"no args uses default", nil, defaultDocsPort, false},
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
			port, err := parseBakeArgs(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseBakeArgs(%v) error = nil, want an error", tt.args)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseBakeArgs(%v) error = %v, want nil", tt.args, err)
			}
			if port != tt.wantPort {
				t.Errorf("parseBakeArgs(%v) = %d, want %d", tt.args, port, tt.wantPort)
			}
		})
	}
}

// TestServeDocumentationServesRealContent binds an OS-assigned
// ephemeral port (":0"), starts serving in the background, makes a
// real HTTP request against it, and confirms both the banner text
// (stdout) and the actual docsite content (internal/docsite's own
// tests cover its content in depth; this just confirms bake.go wires
// it up and actually serves it over the network) show up. Closing the
// listener afterward is what makes http.Serve return deterministically
// — the standard way to end a blocking server loop in a test without
// leaving a goroutine running past it.
func TestServeDocumentationServesRealContent(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() { done <- serveDocumentation(ln, &stdout, &stderr) }()

	// serveDocumentation prints "localhost", not the listener's own
	// 127.0.0.1 address string -- build the request URL the same way
	// so this matches what actually gets printed.
	url := fmt.Sprintf("http://localhost:%d/", port)
	var resp *http.Response
	for i := 0; i < 100; i++ {
		resp, err = http.Get(url)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	body, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		t.Fatalf("reading response body: %v", readErr)
	}
	if !strings.Contains(string(body), "cRust") {
		t.Errorf("response body missing expected content: %q", truncate(string(body), 200))
	}

	ln.Close()
	code := <-done
	if code != 1 {
		t.Errorf("serveDocumentation() after Close() = %d, want 1 (http.Serve reports the closed listener as an error)", code)
	}
	if !strings.Contains(stdout.String(), "Fresh out of the oven at "+url) {
		t.Errorf("stdout = %q, want it to mention the real bound URL %q", stdout.String(), url)
	}
	if !strings.Contains(stderr.String(), "server error") {
		t.Errorf("stderr = %q, want a server error mentioned after Close()", stderr.String())
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func TestRunBakeDocumentationListenFailureReportsError(t *testing.T) {
	// Occupy a port first, then ask runBakeDocumentation for that exact
	// same one -- binding it twice is what forces the net.Listen error
	// path, the one part of runBakeDocumentation serveDocumentation's
	// own test above can't reach (it's passed an already-open listener).
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	var stdout, stderr bytes.Buffer
	code := runBakeDocumentation(port, &stdout, &stderr)
	if code != 1 {
		t.Errorf("runBakeDocumentation() on an already-bound port = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "cannot serve on port") {
		t.Errorf("stderr = %q, want it to mention the bind failure", stderr.String())
	}
}

// TestRunBakeDocumentationSuccessPathBindsAndServes exercises
// runBakeDocumentation's success path (net.Listen succeeding, then
// handing off to serveDocumentation) -- the "listener already taken"
// failure path above covers the error branch, but nothing yet
// confirmed the happy path actually binds and starts serving rather
// than, say, erroring out immediately for some other reason. Port 0
// asks the OS for whichever port is free, so this never collides with
// a real crust bake documentation the developer running these tests
// might already have open. There's no clean way to stop a blocking
// http.Serve loop without a reference to its listener (which
// runBakeDocumentation doesn't expose, unlike serveDocumentation's own
// test above), so this just confirms it's still running after a short
// wait rather than waiting for it to finish.
func TestRunBakeDocumentationSuccessPathBindsAndServes(t *testing.T) {
	done := make(chan int, 1)
	go func() { done <- runBakeDocumentation(0, io.Discard, io.Discard) }()
	select {
	case code := <-done:
		t.Fatalf("runBakeDocumentation(0, ...) returned %d immediately, want it to block serving", code)
	case <-time.After(100 * time.Millisecond):
		// Still running after a short wait -- exactly what a
		// successful bind-and-serve should look like.
	}
}

func TestRunBakeUnknownSubcommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"bake"}, nil, &stdout, &stderr, false)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), `expected "documentation"`) {
		t.Errorf("stderr = %q, want it to mention the missing documentation subcommand", stderr.String())
	}
}

func TestRunBakeDocumentationBadPortFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"bake", "documentation", "--port", "not-a-number"}, nil, &stdout, &stderr, false)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "invalid port") {
		t.Errorf("stderr = %q, want it to mention the invalid port", stderr.String())
	}
}
