package lsp

import "testing"

func TestApplyContentChangeFullDocumentReplace(t *testing.T) {
	got := applyContentChange("x = 1\n", contentChange{Text: "x = 2\n"}, "utf-16")
	if got != "x = 2\n" {
		t.Errorf("got %q, want %q", got, "x = 2\n")
	}
}

func TestApplyContentChangeIncrementalInsert(t *testing.T) {
	// "x = 1\n" -> insert "10" right after the "1" (line 0, col 5..5).
	text := "x = 1\n"
	change := contentChange{
		Range: &Range{Start: Position{Line: 0, Character: 5}, End: Position{Line: 0, Character: 5}},
		Text:  "0",
	}
	got := applyContentChange(text, change, "utf-16")
	if got != "x = 10\n" {
		t.Errorf("got %q, want %q", got, "x = 10\n")
	}
}

func TestApplyContentChangeIncrementalReplace(t *testing.T) {
	// Replace "1" with "42" on line 0 (col 4..5).
	text := "x = 1\n"
	change := contentChange{
		Range: &Range{Start: Position{Line: 0, Character: 4}, End: Position{Line: 0, Character: 5}},
		Text:  "42",
	}
	got := applyContentChange(text, change, "utf-16")
	if got != "x = 42\n" {
		t.Errorf("got %q, want %q", got, "x = 42\n")
	}
}

func TestApplyContentChangeIncrementalDelete(t *testing.T) {
	// Delete "1" entirely (empty replacement text).
	text := "x = 1\n"
	change := contentChange{
		Range: &Range{Start: Position{Line: 0, Character: 4}, End: Position{Line: 0, Character: 5}},
		Text:  "",
	}
	got := applyContentChange(text, change, "utf-16")
	if got != "x = \n" {
		t.Errorf("got %q, want %q", got, "x = \n")
	}
}

func TestApplyContentChangeIncrementalSpansMultipleLines(t *testing.T) {
	text := "x = 1\ny = 2\nz = 3\n"
	// Replace from end of line 0 through start of "3" on line 2.
	change := contentChange{
		Range: &Range{Start: Position{Line: 0, Character: 5}, End: Position{Line: 2, Character: 4}},
		Text:  "9\nw = 8",
	}
	got := applyContentChange(text, change, "utf-16")
	want := "x = 19\nw = 83\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestApplyContentChangeSequentialChangesCompose(t *testing.T) {
	// Two incremental changes applied in order, each relative to the
	// result of the previous one — the way handleDidChange applies a
	// whole contentChanges array.
	text := "x = 1\n"
	changes := []contentChange{
		{Range: &Range{Start: Position{Line: 0, Character: 5}, End: Position{Line: 0, Character: 5}}, Text: "0"},
		{Range: &Range{Start: Position{Line: 0, Character: 6}, End: Position{Line: 0, Character: 6}}, Text: "0"},
	}
	for _, c := range changes {
		text = applyContentChange(text, c, "utf-16")
	}
	if text != "x = 100\n" {
		t.Errorf("got %q, want %q", text, "x = 100\n")
	}
}

func TestApplyContentChangeUTF8Encoding(t *testing.T) {
	// "café" — é is 2 bytes in UTF-8, so a utf-8-encoded Character of 4
	// lands right after "café" (c-a-f-é = 1+1+1+2 = 5 bytes, but the
	// client's own utf-8 "character" units count bytes directly, so
	// Character 5 is the position right after é).
	text := "café\n"
	change := contentChange{
		Range: &Range{Start: Position{Line: 0, Character: 5}, End: Position{Line: 0, Character: 5}},
		Text:  "!",
	}
	got := applyContentChange(text, change, "utf-8")
	if got != "café!\n" {
		t.Errorf("got %q, want %q", got, "café!\n")
	}
}

func TestPositionToByteOffsetClampsOutOfRange(t *testing.T) {
	lines := splitLines("x = 1\ny = 2\n")
	// A line index far past the end should clamp rather than panic.
	off := positionToByteOffset(lines, Position{Line: 100, Character: 0}, "utf-16")
	if off < 0 || off > len("x = 1\ny = 2\n") {
		t.Errorf("got out-of-bounds offset %d", off)
	}

	off = positionToByteOffset(lines, Position{Line: -5, Character: 0}, "utf-16")
	if off != 0 {
		t.Errorf("got %d, want 0 for a negative line", off)
	}
}
