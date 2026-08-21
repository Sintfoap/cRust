// `crust game [file.crust] [-p PORT]` — serve a live, in-browser cRust
// game sandbox over HTTP and open it in a browser, the same shape as
// `crust bake playground` (cmd/crust/playground.go) but backed by a
// different WASM build (cmd/wasmgame, crust-game.wasm) and a
// PixiJS-rendered stage instead of a run-to-completion text pane — see
// cmd/wasmgame/main.go's own doc comment for why a game needs a
// genuinely different execution model (persistent interpreter, one
// call per animation frame) rather than reusing crustRun. An optional
// file argument preloads that file's content into the in-page editor
// (served via a small /source.json endpoint rather than templated
// into index.html itself, so the static HTML/JS never needs escaping
// logic for arbitrary source text) — omit it and the page starts from
// a small built-in demo (arrow keys move a square) instead.
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
)

//go:embed game_assets
var gameAssetsFS embed.FS

// defaultGamePort sits one above bake playground's own default, the
// same spacing every other `crust bake`/`crust game` default already
// keeps so two of these can run at their defaults simultaneously.
const defaultGamePort = 4749

// parseGameArgs reads an optional leading file path and the usual
// -p/--port flag. Unlike bake playground (which never takes a file at
// all), game accepts at most one -- a second bare argument is an
// error, the same "one file, don't guess which one you meant" posture
// every other file-taking subcommand already has.
func parseGameArgs(args []string) (path string, port int, err error) {
	port = defaultGamePort
	for i := 0; i < len(args); i++ {
		a := args[i]
		var val string
		switch {
		case a == "-p" || a == "--port":
			i++
			if i >= len(args) {
				return "", 0, fmt.Errorf("%s requires a port number", a)
			}
			val = args[i]
		case strings.HasPrefix(a, "-p="):
			val = a[len("-p="):]
		case strings.HasPrefix(a, "--port="):
			val = a[len("--port="):]
		case strings.HasPrefix(a, "-"):
			return "", 0, fmt.Errorf("unknown flag %q", a)
		default:
			if path != "" {
				return "", 0, fmt.Errorf("crust game takes at most one file argument (got both %q and %q)", path, a)
			}
			path = a
			continue
		}
		n, convErr := strconv.Atoi(val)
		if convErr != nil || n < 1 || n > 65535 {
			return "", 0, fmt.Errorf("invalid port %q (want 1-65535)", val)
		}
		port = n
	}
	return path, port, nil
}

// gameHandler serves game_assets at the site root (index.html,
// wasm_exec.js, crust-game.wasm), the same fs.Sub-over-embed.FS shape
// playgroundHandler already uses, plus one addition: /source.json,
// which isn't a static asset at all -- it's generated per request from
// whatever file crust game was pointed at (or "" if none), so
// index.html's own JS can preload the editor without any HTML
// templating/escaping.
func gameHandler(source string) http.Handler {
	sub, err := fs.Sub(gameAssetsFS, "game_assets")
	if err != nil {
		// game_assets is a compile-time go:embed of a directory this
		// same package ships, so this can't fail in a built binary --
		// panicking would just mean the embed itself is broken, same
		// class of bug as a missing embed tag.
		panic(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/source.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"source": source})
	})
	return mux
}

// runGame reads path (if given), binds port, and hands off to
// serveGame -- split the same way runBakePlayground/servePlayground
// already are, so a test can exercise the actual serving against a
// listener it created and controls directly.
func runGame(path string, port int, stdout, stderr io.Writer) int {
	var source string
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(stderr, "crust game: %s\n", err)
			return 1
		}
		source = string(data)
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		fmt.Fprintf(stderr, "crust game: cannot serve on port %d: %v\n", port, err)
		return 1
	}
	return serveGame(ln, source, stdout, stderr)
}

// serveGame prints where to reach the sandbox, opens a browser
// best-effort, and serves until ln is closed or the process is
// interrupted.
func serveGame(ln net.Listener, source string, stdout, stderr io.Writer) int {
	addr, _ := ln.Addr().(*net.TCPAddr)
	url := fmt.Sprintf("http://localhost:%d/", addr.Port)
	fmt.Fprintln(stdout, "🍕 crust game")
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "  Fresh out of the oven at %s\n", url)
	fmt.Fprintln(stdout, "  rect/circle/setPos/keyDown/onFrame, rendered live with PixiJS.")
	fmt.Fprintln(stdout, "  Press Ctrl+C when you've had enough.")
	fmt.Fprintln(stdout)

	openBrowser(url, stdout)

	if err := http.Serve(ln, gameHandler(source)); err != nil {
		fmt.Fprintf(stderr, "crust game: server error: %v\n", err)
		return 1
	}
	return 0
}
