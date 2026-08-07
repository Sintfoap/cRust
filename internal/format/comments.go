// Comment handling: `//` line comments are stripped by the lexer
// before the parser ever sees them (internal/lexer's skipLineComment)
// — they never become tokens, so they're completely invisible to the
// AST. A formatter that only ever re-prints the AST would therefore
// silently delete every comment in the file, which is not an
// acceptable trade for prettier output. scanComments does a small,
// standalone pass over the raw source (independent of
// internal/lexer/internal/parser) to recover comment text and
// position, so the printer can weave them back into its output.
package format

import "strings"

// Comment is one `//...` line comment recovered from source, with
// enough information for the printer to place it correctly: which
// line it was on, its text (including the leading `//`, trimmed of
// trailing whitespace but otherwise verbatim — so any internal
// spacing/punctuation the author chose survives), and whether it
// shared its line with real code (Standalone false) or had a whole
// line to itself (Standalone true) — the two cases print differently
// (see printer.go's flushComments/trailingComment).
type Comment struct {
	Line       int
	Text       string
	Standalone bool
}

// scanComments walks src once, tracking only what it needs to tell a
// genuine `//` comment start from one that merely appears inside a
// String literal (e.g. a URL like "http://example.com") — mirroring
// just enough of internal/lexer's own string-escape handling (`\"` and
// `\\` specifically) to not be fooled by a quote inside an escape.
// Unlike the real lexer this never reports an error: a source file
// that doesn't actually lex/parse cleanly never reaches this function
// in the first place (Format's caller already needs a successful parse
// to have an *ast.Program to print), so scanComments only ever needs
// to be correct on valid cRust source.
func scanComments(src string) []Comment {
	var comments []Comment
	runes := []rune(src)
	n := len(runes)

	line := 1
	sawCodeOnLine := false
	inString := false

	for i := 0; i < n; i++ {
		ch := runes[i]

		if inString {
			switch ch {
			case '\\':
				i++ // skip whatever's escaped (including a `\"`)
			case '"':
				inString = false
			case '\n':
				// An unterminated string; the real lexer would flag
				// this as ILLEGAL and parsing would already have
				// failed before scanComments is ever called. Bail out
				// of string mode rather than misreading the rest of
				// the file as string content.
				inString = false
				line++
				sawCodeOnLine = false
			}
			continue
		}

		switch {
		case ch == '"':
			inString = true
			sawCodeOnLine = true
		case ch == '\n':
			line++
			sawCodeOnLine = false
		case ch == '/' && i+1 < n && runes[i+1] == '/':
			start := i
			for i < n && runes[i] != '\n' {
				i++
			}
			text := strings.TrimRight(string(runes[start:i]), " \t\r")
			comments = append(comments, Comment{Line: line, Text: text, Standalone: !sawCodeOnLine})
			if i < n { // landed on '\n'; the outer loop's i++ will pass it, so account for it here
				line++
				sawCodeOnLine = false
			}
		case ch != ' ' && ch != '\t' && ch != '\r':
			sawCodeOnLine = true
		}
	}

	return comments
}
