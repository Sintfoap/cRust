package lsp

// applyContentChange applies one textDocument/didChange contentChanges
// entry to text, returning the new document. change.Range == nil means
// a full-document replace (change.Text is the entire new document,
// same as this package's original TextDocumentSyncKind.Full behavior);
// otherwise change.Text replaces exactly the span change.Range covers,
// per LSP's TextDocumentContentChangeEvent — the actual incremental
// edit. encoding is the negotiated position encoding (position.go),
// since change.Range's Positions are expressed in the client's chosen
// units, not raw bytes.
func applyContentChange(text string, change contentChange, encoding string) string {
	if change.Range == nil {
		return change.Text
	}
	lines := splitLines(text)
	start := positionToByteOffset(lines, change.Range.Start, encoding)
	end := positionToByteOffset(lines, change.Range.End, encoding)
	return text[:start] + change.Text + text[end:]
}

// positionToByteOffset converts an LSP Position (0-indexed line, plus
// a Character in encoding's units — position.go's decodeOffset already
// handles the unit conversion within one line) into a byte offset into
// the full document lines were split from. Out-of-range inputs clamp
// to the nearest valid offset (start or end of document) rather than
// panicking — a client sending a stale Range against text that's
// already moved on (a race between two rapid edits) should degrade
// gracefully, not crash the server.
func positionToByteOffset(lines []string, pos Position, encoding string) int {
	lineIdx := pos.Line
	if lineIdx < 0 {
		lineIdx = 0
	}
	if lineIdx >= len(lines) {
		lineIdx = len(lines) - 1
	}
	if lineIdx < 0 {
		return 0
	}

	offset := 0
	for i := 0; i < lineIdx; i++ {
		offset += len(lines[i]) + 1 // +1 for the '\n' splitLines split on
	}

	line := lines[lineIdx]
	runeCol := decodeOffset(line, pos.Character, encoding)
	offset += encodeOffset(line, runeCol, "utf-8") // rune count -> byte count within the line
	return offset
}
