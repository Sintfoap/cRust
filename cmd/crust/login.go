// `crust login` — save the adventofcode.com session cookie that
// `crust fetch` (and `crust develop`'s own auto-fetch) need to
// download puzzle input on the caller's behalf. The cookie comes from
// a browser already logged into AoC (dev tools -> Application/Storage
// -> cookies -> "session") — crust has no login flow of its own, AoC
// doesn't offer one outside a browser.
package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/Sintfoap/cRust/internal/aoc"
)

// runLogin prompts for the session cookie on stdin and saves it via
// aoc.SaveSession. No masking: golang.org/x/term isn't a dependency
// this project already carries, and adding one purely to hide a
// single paste didn't seem worth it — the prompt says so up front so
// it's never a surprise.
func runLogin(stdin io.Reader, stdout, stderr io.Writer) int {
	fmt.Fprintln(stdout, "Paste your adventofcode.com session cookie (from a logged-in browser's")
	fmt.Fprintln(stdout, "dev tools -> Application/Storage -> Cookies -> \"session\").")
	fmt.Fprintln(stdout, "Note: this will be visible as you type/paste it — there's no input masking.")
	fmt.Fprint(stdout, "session: ")

	scanner := bufio.NewScanner(stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(stderr, "crust login: %s\n", err)
			return 1
		}
		fmt.Fprintln(stderr, "crust login: no input received")
		return 1
	}

	session := strings.TrimSpace(scanner.Text())
	if session == "" {
		fmt.Fprintln(stderr, "crust login: empty session cookie")
		return 1
	}

	if err := aoc.SaveSession(session); err != nil {
		fmt.Fprintf(stderr, "crust login: saving session: %s\n", err)
		return 1
	}

	fmt.Fprintln(stdout, "Session saved. `crust fetch <day>` will use it from now on.")
	return 0
}
