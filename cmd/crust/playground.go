// `crust bake playground` — serve a self-contained, in-browser cRust
// sandbox over HTTP and open it in a browser:
//
//	crust bake playground            # serve on http://localhost:4748
//	crust bake playground -p 8080    # serve on http://localhost:8080
//
// The page runs cRust entirely client-side via WebAssembly
// (playground_assets/crust.wasm, built by scripts/build-wasm.sh from
// cmd/wasm and embedded into the binary at compile time) — nothing
// typed into it is sent anywhere, and it works from any built binary
// with no source tree or Go toolchain present, same reasoning as `bake
// documentation`.
package main

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"
)

//go:embed playground_assets
var playgroundAssetsFS embed.FS

// defaultPlaygroundPort is where `bake playground` serves when no -p is
// given. One above defaultDocsPort so `bake documentation` and `bake
// playground` can both run at their defaults at the same time.
const defaultPlaygroundPort = 4748

// parsePlaygroundArgs reads the optional `-p PORT` / `--port PORT` flag
// (also accepting the `-p=PORT`/`--port=PORT` form). `bake playground`
// takes no file, so any bare argument is an error.
func parsePlaygroundArgs(args []string) (int, error) {
	port := defaultPlaygroundPort
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
			return 0, fmt.Errorf("unknown flag %q (bake playground accepts only -p/--port)", a)
		default:
			return 0, fmt.Errorf("bake playground takes no file argument (got %q)", a)
		}
		n, err := strconv.Atoi(val)
		if err != nil || n < 1 || n > 65535 {
			return 0, fmt.Errorf("invalid port %q (want 1-65535)", val)
		}
		port = n
	}
	return port, nil
}

// playgroundHandler serves playground_assets at the site root (index.html,
// wasm_exec.js, crust.wasm) instead of under a /playground_assets/ prefix.
func playgroundHandler() http.Handler {
	sub, err := fs.Sub(playgroundAssetsFS, "playground_assets")
	if err != nil {
		// playground_assets is a compile-time go:embed of a directory
		// this same package ships, so this can't fail in a built
		// binary — panicking would just mean the embed itself is
		// broken, same class of bug as a missing embed tag.
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}

// runBakePlayground binds port and hands off to servePlayground. Split
// from it specifically so a test can exercise the actual serving
// against a listener it created and controls directly — same reasoning
// as bake.go's runBakeDocumentation/serveDocumentation split.
func runBakePlayground(port int, stdout, stderr io.Writer) int {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		fmt.Fprintf(stderr, "crust bake playground: cannot serve on port %d: %v\n", port, err)
		return 1
	}
	return servePlayground(ln, stdout, stderr)
}

// servePlayground prints where to reach the playground, opens a browser
// best-effort, and serves until ln is closed or the process is
// interrupted. It blocks; Ctrl+C ends it in normal use, same as every
// other long-running `crust` subcommand.
func servePlayground(ln net.Listener, stdout, stderr io.Writer) int {
	addr, _ := ln.Addr().(*net.TCPAddr)
	url := fmt.Sprintf("http://localhost:%d/", addr.Port)
	fmt.Fprintln(stdout, "🍕 crust bake: playground")
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "  Fresh out of the oven at %s\n", url)
	fmt.Fprintln(stdout, "  Everything runs client-side — nothing you type is sent anywhere.")
	fmt.Fprintln(stdout, "  Press Ctrl+C when you've had enough.")
	fmt.Fprintln(stdout)

	openBrowser(url, stdout)

	if err := http.Serve(ln, playgroundHandler()); err != nil {
		fmt.Fprintf(stderr, "crust bake playground: server error: %v\n", err)
		return 1
	}
	return 0
}
