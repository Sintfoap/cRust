package lsp

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
)

func TestFormatDocumentReturnsWholeDocumentEdit(t *testing.T) {
	src := "x=1\ndeliver(x)\n"
	edits := formatDocument(src, "utf-16")
	if len(edits) != 1 {
		t.Fatalf("len(edits) = %d, want 1", len(edits))
	}
	e := edits[0]
	want := "x = 1\ndeliver(x)\n"
	if e.NewText != want {
		t.Errorf("NewText = %q, want %q", e.NewText, want)
	}
	if e.Range.Start != (Position{Line: 0, Character: 0}) {
		t.Errorf("Range.Start = %+v, want the document start", e.Range.Start)
	}
	// src ends in "\n", so splitLines reports a trailing empty final
	// line (splitLines' own documented behavior) -- the end position
	// covering the whole document is therefore the start of that empty
	// line (index 2: "x=1", "deliver(x)", ""), not the end of
	// "deliver(x)" itself.
	wantEnd := Position{Line: 2, Character: 0}
	if e.Range.End != wantEnd {
		t.Errorf("Range.End = %+v, want %+v", e.Range.End, wantEnd)
	}
}

func TestFormatDocumentAlreadyCanonicalReturnsEmptyEdits(t *testing.T) {
	src := "x = 1\ndeliver(x)\n"
	edits := formatDocument(src, "utf-16")
	if edits == nil {
		t.Fatal("formatDocument() = nil, want a non-nil empty slice for an already-canonical document")
	}
	if len(edits) != 0 {
		t.Errorf("len(edits) = %d, want 0 (no change needed)", len(edits))
	}
}

func TestFormatDocumentParseErrorReturnsNil(t *testing.T) {
	edits := formatDocument("x = (\n", "utf-16")
	if edits != nil {
		t.Errorf("formatDocument() on unparseable source = %v, want nil", edits)
	}
}

func TestFormatDocumentPreservesComments(t *testing.T) {
	src := "// keep me\nx=1\n"
	edits := formatDocument(src, "utf-16")
	if len(edits) != 1 {
		t.Fatalf("len(edits) = %d, want 1", len(edits))
	}
	if edits[0].NewText != "// keep me\nx = 1\n" {
		t.Errorf("NewText = %q, want the comment preserved", edits[0].NewText)
	}
}

// TestServerFormattingViaJSONRPC exercises the full request/response
// path through Server.Run over real wire bytes -- the same discipline
// lsp_test.go's TestServerSessionRichFeatures uses for every other
// rich feature -- confirming initialize actually advertises the
// capability and a real textDocument/formatting request gets a real
// edit back.
func TestServerFormattingViaJSONRPC(t *testing.T) {
	var input bytes.Buffer
	input.Write(clientMessage(t, "initialize", 1, map[string]any{"capabilities": map[string]any{}}))
	input.Write(clientMessage(t, "textDocument/didOpen", nil, map[string]any{
		"textDocument": map[string]any{"uri": uri, "languageId": "crust", "version": 1, "text": "x=1\ndeliver(x)\n"},
	}))
	input.Write(clientMessage(t, "textDocument/formatting", 2, map[string]any{
		"textDocument": map[string]any{"uri": uri},
	}))
	input.Write(clientMessage(t, "exit", nil, nil))

	var output bytes.Buffer
	s := NewServer()
	if err := s.Run(&input, &output, io.Discard); err != nil {
		t.Fatalf("Run: %s", err)
	}
	msgs := decodeServerMessages(t, output.Bytes())
	// initialize response, publishDiagnostics (from didOpen), formatting response.
	if len(msgs) != 3 {
		t.Fatalf("got %d messages, want 3: %+v", len(msgs), msgs)
	}

	var initResult initializeResult
	if err := json.Unmarshal(msgs[0].Result, &initResult); err != nil {
		t.Fatalf("unmarshal initialize result: %s", err)
	}
	if !initResult.Capabilities.DocumentFormattingProvider {
		t.Error("initialize result: DocumentFormattingProvider = false, want true")
	}

	var edits []TextEdit
	if err := json.Unmarshal(msgs[2].Result, &edits); err != nil {
		t.Fatalf("unmarshal formatting result: %s", err)
	}
	if len(edits) != 1 {
		t.Fatalf("len(edits) = %d, want 1", len(edits))
	}
	if edits[0].NewText != "x = 1\ndeliver(x)\n" {
		t.Errorf("NewText = %q, want the formatted source", edits[0].NewText)
	}
}

// TestServerFormattingUnknownDocumentReturnsNilResult confirms
// formatting a document the server never saw a didOpen for degrades to
// a null result, the same "nothing to answer with" convention hover/
// definition/etc. already use, rather than an error.
func TestServerFormattingUnknownDocumentReturnsNilResult(t *testing.T) {
	var input bytes.Buffer
	input.Write(clientMessage(t, "textDocument/formatting", 1, map[string]any{
		"textDocument": map[string]any{"uri": "file:///never-opened.crust"},
	}))
	input.Write(clientMessage(t, "exit", nil, nil))

	var output bytes.Buffer
	s := NewServer()
	if err := s.Run(&input, &output, io.Discard); err != nil {
		t.Fatalf("Run: %s", err)
	}
	msgs := decodeServerMessages(t, output.Bytes())
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1: %+v", len(msgs), msgs)
	}
	if string(msgs[0].Result) != "null" {
		t.Errorf("result = %s, want null for a document that was never opened", msgs[0].Result)
	}
}
