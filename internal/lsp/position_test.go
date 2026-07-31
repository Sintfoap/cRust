package lsp

import "testing"

// TestEncodeOffsetASCII checks the trivial case: for pure ASCII, all
// three encodings agree (1 unit per rune).
func TestEncodeOffsetASCII(t *testing.T) {
	line := "deliver(1)"
	for _, enc := range []string{"utf-8", "utf-16", "utf-32"} {
		if got := encodeOffset(line, 7, enc); got != 7 {
			t.Errorf("encodeOffset(%q, 7, %q) = %d, want 7", line, enc, got)
		}
	}
}

// TestEncodeOffsetMultiByte uses "é" (U+00E9, 2 UTF-8 bytes, 1 UTF-16
// unit, 1 code point) before the target rune, so utf-8 should diverge
// from utf-16/utf-32.
func TestEncodeOffsetMultiByte(t *testing.T) {
	line := "éx" // é = 2 bytes in UTF-8, 1 rune
	// runeCol 1 means "after the first rune (é)".
	if got := encodeOffset(line, 1, "utf-8"); got != 2 {
		t.Errorf("utf-8 offset = %d, want 2", got)
	}
	if got := encodeOffset(line, 1, "utf-16"); got != 1 {
		t.Errorf("utf-16 offset = %d, want 1", got)
	}
	if got := encodeOffset(line, 1, "utf-32"); got != 1 {
		t.Errorf("utf-32 offset = %d, want 1", got)
	}
}

// TestEncodeOffsetAstral uses "🍕" (U+1F355, 4 UTF-8 bytes, 2 UTF-16
// surrogate-pair units, 1 code point) — the case that actually
// motivates supporting more than one encoding at all.
func TestEncodeOffsetAstral(t *testing.T) {
	line := "🍕x"
	if got := encodeOffset(line, 1, "utf-8"); got != 4 {
		t.Errorf("utf-8 offset = %d, want 4", got)
	}
	if got := encodeOffset(line, 1, "utf-16"); got != 2 {
		t.Errorf("utf-16 offset = %d, want 2", got)
	}
	if got := encodeOffset(line, 1, "utf-32"); got != 1 {
		t.Errorf("utf-32 offset = %d, want 1", got)
	}
}

// TestDecodeOffsetRoundTrip confirms decodeOffset undoes encodeOffset
// for every rune boundary in a mixed ASCII/astral line, for each
// encoding.
func TestDecodeOffsetRoundTrip(t *testing.T) {
	line := "a🍕bé"
	runes := []rune(line)
	for _, enc := range []string{"utf-8", "utf-16", "utf-32"} {
		for runeCol := 0; runeCol <= len(runes); runeCol++ {
			encoded := encodeOffset(line, runeCol, enc)
			decoded := decodeOffset(line, encoded, enc)
			if decoded != runeCol {
				t.Errorf("[%s] decodeOffset(encodeOffset(%d)) = %d, want %d", enc, runeCol, decoded, runeCol)
			}
		}
	}
}

func TestNegotiateEncoding(t *testing.T) {
	tests := []struct {
		name string
		gen  *generalClientCapabilities
		want string
	}{
		{"no capability", nil, "utf-16"},
		{"empty list", &generalClientCapabilities{PositionEncodings: []string{}}, "utf-16"},
		{"prefers client order", &generalClientCapabilities{PositionEncodings: []string{"utf-32", "utf-8"}}, "utf-32"},
		{"skips unsupported", &generalClientCapabilities{PositionEncodings: []string{"utf-7", "utf-8"}}, "utf-8"},
		{"utf-16 only", &generalClientCapabilities{PositionEncodings: []string{"utf-16"}}, "utf-16"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := negotiateEncoding(tt.gen); got != tt.want {
				t.Errorf("negotiateEncoding(%+v) = %q, want %q", tt.gen, got, tt.want)
			}
		})
	}
}

func TestToPosition(t *testing.T) {
	lines := []string{"deliver(1)", "🍕rest"}
	// 1-indexed (line=2, col=2) is the rune right after 🍕 on line 2.
	got := toPosition(lines, 2, 2, "utf-16")
	want := Position{Line: 1, Character: 2} // 🍕 takes 2 UTF-16 units
	if got != want {
		t.Errorf("toPosition = %+v, want %+v", got, want)
	}
}
