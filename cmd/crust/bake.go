// `crust bake documentation` — serve the browsable documentation
// website (internal/docsite, embedded into the binary at build time)
// over HTTP and open it in a browser:
//
//	crust bake documentation            # serve on http://localhost:4747
//	crust bake documentation -p 8080    # serve on http://localhost:8080
//
// Because the site is embedded, this works from any built binary, with
// no source tree present — same reasoning as every other embedded-FS
// feature in this codebase (editors/tree-sitter-crust's generated
// parser, docs/embed.go equivalents elsewhere).
package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/Sintfoap/cRust/internal/docsite"
)

// defaultDocsPort is where `bake documentation` serves when no -p is
// given — an arbitrary high, rarely-used port, chosen only to avoid
// colliding with common dev-server defaults (3000, 8000, 8080, ...).
const defaultDocsPort = 4747

// parseBakeArgs reads the optional `-p PORT` / `--port PORT` flag
// (also accepting the `-p=PORT`/`--port=PORT` form). `bake
// documentation` takes no file, so any bare argument is an error.
func parseBakeArgs(args []string) (int, error) {
	port := defaultDocsPort
	for i := 0; i < len(args); i++ {
		a := args[i]
		var val string
		switch {
		case a == "-p" || a == "--port":
			i++
			if i >= len(args) {
				return 0, fmt.Errorf("%s requires a port number", a)
			}
			val = args[i]
		case strings.HasPrefix(a, "-p="):
			val = a[len("-p="):]
		case strings.HasPrefix(a, "--port="):
			val = a[len("--port="):]
		case strings.HasPrefix(a, "-"):
			return 0, fmt.Errorf("unknown flag %q (bake documentation accepts only -p/--port)", a)
		default:
			return 0, fmt.Errorf("bake documentation takes no file argument (got %q)", a)
		}
		n, err := strconv.Atoi(val)
		if err != nil || n < 1 || n > 65535 {
			return 0, fmt.Errorf("invalid port %q (want 1-65535)", val)
		}
		port = n
	}
	return port, nil
}

// runBakeDocumentation binds port and hands off to serveDocumentation.
// Split from it specifically so a test can exercise the actual serving
// (banner text, real HTTP responses, shutdown behavior) against a
// listener it created and controls directly — binding "127.0.0.1:0"
// for an OS-assigned ephemeral port and closing it deterministically —
// without needing to know or guess which real port a fixed one like
// defaultDocsPort resolves to, or leave a goroutine serving forever
// past the end of that test.
func runBakeDocumentation(port int, stdout, stderr io.Writer) int {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		fmt.Fprintf(stderr, "crust bake documentation: cannot serve on port %d: %v\n", port, err)
		return 1
	}
	return serveDocumentation(ln, stdout, stderr)
}

// serveDocumentation prints where to reach the site (reading the
// address back off ln rather than trusting a separately-tracked port
// number, so this is correct even for an OS-assigned ephemeral port),
// opens a browser best-effort, and serves until ln is closed or the
// process is interrupted. It blocks; Ctrl+C ends it in normal use (the
// OS's default SIGINT handling, same as every other long-running
// `crust` subcommand — nothing here needs its own signal handler).
func serveDocumentation(ln net.Listener, stdout, stderr io.Writer) int {
	addr, _ := ln.Addr().(*net.TCPAddr)
	url := fmt.Sprintf("http://localhost:%d/", addr.Port)
	fmt.Fprintln(stdout, "🍕 crust bake: documentation")
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "  Fresh out of the oven at %s\n", url)
	fmt.Fprintln(stdout, "  Press Ctrl+C when you've had enough.")
	fmt.Fprintln(stdout)

	openBrowser(url, stdout)

	if err := http.Serve(ln, docsite.Handler()); err != nil {
		fmt.Fprintf(stderr, "crust bake documentation: server error: %v\n", err)
		return 1
	}
	return 0
}

// openBrowser launches the platform's default browser at url. Best-
// effort courtesy: if no opener is available (a headless server, or a
// minimal container without xdg-open on PATH), the URL was already
// printed above, so a failure here is just noted, never fatal.
func openBrowser(url string, stdout io.Writer) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		fmt.Fprintln(stdout, "  (couldn't open a browser automatically — visit the URL above)")
		return
	}
	// Reap the opener so it doesn't linger as a zombie; whether it
	// actually succeeded doesn't change anything here — the URL is
	// already on-screen either way.
	go func() { _ = cmd.Wait() }()
}
