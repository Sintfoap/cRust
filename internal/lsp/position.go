package lsp

import (
	"unicode/utf16"
	"unicode/utf8"
)

// splitLines splits text into lines on '\n', the same terminator
// internal/lexer counts lines by. Every caller here only uses the
// result to measure runes before a given column, never to display the
// line, so a trailing '\r' from CRLF input just rides along as part of
// whichever rune count it falls into — harmless.
func splitLines(text string) []string {
	lines := []string{}
	start := 0
	for i, r := range text {
		if r == '\n' {
			lines = append(lines, text[start:i])
			start = i + 1
		}
	}
	lines = append(lines, text[start:])
	return lines
}

// toPosition converts a 1-indexed (line, col) — col counted in runes,
// matching internal/lexer and internal/parser's token positions — into
// an LSP Position, with Character encoded per encoding ("utf-8",
// "utf-16", or "utf-32", as negotiated in initialize — see
// negotiateEncoding).
func toPosition(lines []string, line, col int, encoding string) Position {
	lineIdx := line - 1
	if lineIdx < 0 {
		lineIdx = 0
	}
	var lineText string
	if lineIdx < len(lines) {
		lineText = lines[lineIdx]
	}
	runeCol := col - 1
	if runeCol < 0 {
		runeCol = 0
	}
	return Position{Line: lineIdx, Character: encodeOffset(lineText, runeCol, encoding)}
}

// encodeOffset returns how far into line the runeCol-th rune sits,
// measured in the units LSP's PositionEncodingKind names: "utf-8"
// counts UTF-8 code units (bytes), "utf-16" counts UTF-16 code units
// (2 per astral-plane rune), "utf-32" counts code points (runes)
// directly — which is exactly what internal/lexer already counts, so
// that case is a straight passthrough.
func encodeOffset(line string, runeCol int, encoding string) int {
	if encoding == "utf-32" {
		return runeCol
	}
	offset := 0
	n := 0
	for _, r := range line {
		if n >= runeCol {
			break
		}
		if encoding == "utf-8" {
			offset += utf8.RuneLen(r)
		} else { // "utf-16"
			if rl := utf16.RuneLen(r); rl > 0 {
				offset += rl
			} else {
				offset++
			}
		}
		n++
	}
	return offset
}

// decodeOffset is encodeOffset's inverse: given character (an incoming
// LSP Position.Character, in encoding's units) into line, returns how
// many whole runes precede it — used to turn a client's hover/etc.
// position back into the rune column internal/lexer's tokens are
// positioned in.
func decodeOffset(line string, character int, encoding string) int {
	if encoding == "utf-32" {
		return character
	}
	consumed := 0
	n := 0
	for _, r := range line {
		if consumed >= character {
			break
		}
		if encoding == "utf-8" {
			consumed += utf8.RuneLen(r)
		} else { // "utf-16"
			if rl := utf16.RuneLen(r); rl > 0 {
				consumed += rl
			} else {
				consumed++
			}
		}
		n++
	}
	return n
}

// negotiateEncoding picks a position encoding per the LSP spec: the
// client lists general.positionEncodings in preference order (most
// preferred first); the server picks the first one it supports.
// Absent that capability (an older/simpler client), LSP's documented
// default is "utf-16".
func negotiateEncoding(gen *generalClientCapabilities) string {
	if gen == nil {
		return "utf-16"
	}
	supported := map[string]bool{"utf-8": true, "utf-16": true, "utf-32": true}
	for _, enc := range gen.PositionEncodings {
		if supported[enc] {
			return enc
		}
	}
	return "utf-16"
}
