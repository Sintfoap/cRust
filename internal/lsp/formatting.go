package lsp

import (
	"github.com/Sintfoap/cRust/internal/format"
	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/parser"
)

// formatDocument runs internal/format over text and returns the edit
// needed to turn it into the canonical result — a single TextEdit
// spanning the whole document, since internal/format always reprints
// the entire file rather than computing a minimal diff (the same
// "replace everything" shape most LSP formatters use when they don't
// bother with a line-level diff). Returns an empty (non-nil) slice,
// not an edit, when text is already canonical, so a format-on-save
// binding doesn't touch the file's mtime/undo-history for no reason.
// Returns nil — "no edits, nothing to apply" — for a document that
// doesn't parse; formatting an unparseable file is exactly as
// impossible as running one, and there's no separate error channel a
// formatting response can use to say why the way hover/definition
// silently returning nil already can't either.
func formatDocument(text string, encoding string) []TextEdit {
	l := lexer.New(text)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		return nil
	}

	formatted := format.Format(program, text)
	if formatted == text {
		return []TextEdit{}
	}

	lines := splitLines(text)
	endLine := len(lines)
	endCol := len([]rune(lines[len(lines)-1])) + 1
	fullRange := Range{
		Start: Position{Line: 0, Character: 0},
		End:   toPosition(lines, endLine, endCol, encoding),
	}
	return []TextEdit{{Range: fullRange, NewText: formatted}}
}
