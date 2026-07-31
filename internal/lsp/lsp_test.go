package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"testing"
)

// clientMessage builds one framed JSON-RPC message the way a real
// client would put it on the wire — used to assemble a whole session's
// worth of input as one byte stream, exactly what Server.Run reads.
func clientMessage(t *testing.T, method string, id any, params any) []byte {
	t.Helper()
	m := map[string]any{"jsonrpc": "2.0", "method": method}
	if id != nil {
		m["id"] = id
	}
	if params != nil {
		m["params"] = params
	}
	body, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal client message: %s", err)
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Content-Length: %d\r\n\r\n", len(body))
	buf.Write(body)
	return buf.Bytes()
}

// decodeServerMessages reads every framed message out of a real byte
// stream the way a real client would, using the package's own
// readMessage — so this test round-trips actual bytes end to end
// rather than calling internal handlers directly.
func decodeServerMessages(t *testing.T, raw []byte) []rpcMessage {
	t.Helper()
	var out []rpcMessage
	r := bufio.NewReader(bytes.NewReader(raw))
	for {
		body, err := readMessage(r)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("decodeServerMessages: %s", err)
		}
		var msg rpcMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			t.Fatalf("decodeServerMessages: unmarshal: %s", err)
		}
		out = append(out, msg)
	}
	return out
}

// TestServerSession drives a full, realistic client session —
// initialize, initialized, didOpen a file with a lex error, hover on a
// builtin, shutdown, exit — through Server.Run over real byte streams
// (bytes.Buffer standing in for stdin/stdout, framed exactly like real
// LSP traffic), and checks every response/notification that comes back
// out the other side.
func TestServerSession(t *testing.T) {
	var input bytes.Buffer
	input.Write(clientMessage(t, "initialize", 1, map[string]any{
		"capabilities": map[string]any{
			"general": map[string]any{"positionEncodings": []string{"utf-8", "utf-16"}},
		},
	}))
	input.Write(clientMessage(t, "initialized", nil, map[string]any{}))
	input.Write(clientMessage(t, "textDocument/didOpen", nil, map[string]any{
		"textDocument": map[string]any{
			"uri":        "file:///day01.crust",
			"languageId": "crust",
			"version":    1,
			"text":       "deliver(@)\n",
		},
	}))
	input.Write(clientMessage(t, "textDocument/hover", 2, map[string]any{
		"textDocument": map[string]any{"uri": "file:///day01.crust"},
		"position":     map[string]any{"line": 0, "character": 0},
	}))
	input.Write(clientMessage(t, "shutdown", 3, nil))
	input.Write(clientMessage(t, "exit", nil, nil))

	var output bytes.Buffer
	s := NewServer()
	if err := s.Run(&input, &output, io.Discard); err != nil {
		t.Fatalf("Run: %s", err)
	}

	msgs := decodeServerMessages(t, output.Bytes())
	if len(msgs) != 4 {
		t.Fatalf("got %d server messages, want 4 (initialize response, publishDiagnostics, hover response, shutdown response): %+v", len(msgs), msgs)
	}

	// 1. initialize response: prefers "utf-8" since the client listed
	// it first among encodings we both support.
	var initResult initializeResult
	if err := json.Unmarshal(msgs[0].Result, &initResult); err != nil {
		t.Fatalf("unmarshal initialize result: %s", err)
	}
	if !initResult.Capabilities.HoverProvider {
		t.Error("initialize result: HoverProvider = false, want true")
	}
	if initResult.Capabilities.PositionEncoding != "utf-8" {
		t.Errorf("initialize result: PositionEncoding = %q, want utf-8", initResult.Capabilities.PositionEncoding)
	}

	// 2. publishDiagnostics notification for the illegal '@' character.
	if msgs[1].Method != "textDocument/publishDiagnostics" {
		t.Fatalf("msgs[1].Method = %q, want textDocument/publishDiagnostics", msgs[1].Method)
	}
	var diagParams publishDiagnosticsParams
	if err := json.Unmarshal(msgs[1].Params, &diagParams); err != nil {
		t.Fatalf("unmarshal publishDiagnostics params: %s", err)
	}
	if len(diagParams.Diagnostics) != 1 {
		t.Fatalf("len(diagnostics) = %d, want 1: %+v", len(diagParams.Diagnostics), diagParams.Diagnostics)
	}

	// 3. hover response for "deliver" at (0,0).
	var hover hoverResult
	if err := json.Unmarshal(msgs[2].Result, &hover); err != nil {
		t.Fatalf("unmarshal hover result: %s", err)
	}
	if hover.Contents.Value != builtinDocs["deliver"] {
		t.Errorf("hover value = %q, want %q", hover.Contents.Value, builtinDocs["deliver"])
	}

	// 4. shutdown response: null result, no error.
	if msgs[3].Error != nil {
		t.Errorf("shutdown response has error: %+v", msgs[3].Error)
	}
}

// TestServerSessionRichFeatures drives definition, references,
// documentSymbol, completion, and rename through Server.Run over real
// byte streams — the same "actual wire bytes in, actual wire bytes
// out" discipline as TestServerSession, extended to every method added
// after the initial hover+diagnostics server.
func TestServerSessionRichFeatures(t *testing.T) {
	src := "recipe helper(x) {\n    serve x * 2\n}\n\ntotal = helper(5)\ndeliver(total)\n"
	// "helper" call site: line 4 (0-indexed), right after "total = ".
	helperCallPos := map[string]any{"line": 4, "character": 8}

	var input bytes.Buffer
	input.Write(clientMessage(t, "initialize", 1, map[string]any{"capabilities": map[string]any{}}))
	input.Write(clientMessage(t, "textDocument/didOpen", nil, map[string]any{
		"textDocument": map[string]any{"uri": uri, "languageId": "crust", "version": 1, "text": src},
	}))
	input.Write(clientMessage(t, "textDocument/definition", 2, map[string]any{
		"textDocument": map[string]any{"uri": uri}, "position": helperCallPos,
	}))
	input.Write(clientMessage(t, "textDocument/references", 3, map[string]any{
		"textDocument": map[string]any{"uri": uri}, "position": helperCallPos,
		"context": map[string]any{"includeDeclaration": true},
	}))
	input.Write(clientMessage(t, "textDocument/documentSymbol", 4, map[string]any{
		"textDocument": map[string]any{"uri": uri},
	}))
	input.Write(clientMessage(t, "textDocument/completion", 5, map[string]any{
		"textDocument": map[string]any{"uri": uri}, "position": map[string]any{"line": 0, "character": 0},
	}))
	input.Write(clientMessage(t, "textDocument/rename", 6, map[string]any{
		"textDocument": map[string]any{"uri": uri}, "position": helperCallPos, "newName": "compute",
	}))
	input.Write(clientMessage(t, "exit", nil, nil))

	var output bytes.Buffer
	s := NewServer()
	if err := s.Run(&input, &output, io.Discard); err != nil {
		t.Fatalf("Run: %s", err)
	}
	msgs := decodeServerMessages(t, output.Bytes())
	// initialize response, publishDiagnostics (from didOpen), then one
	// response per request.
	if len(msgs) != 7 {
		t.Fatalf("got %d messages, want 7: %+v", len(msgs), msgs)
	}

	var initResult initializeResult
	if err := json.Unmarshal(msgs[0].Result, &initResult); err != nil {
		t.Fatalf("unmarshal initialize result: %s", err)
	}
	if !initResult.Capabilities.DefinitionProvider || !initResult.Capabilities.TypeDefinitionProvider ||
		!initResult.Capabilities.ReferencesProvider || !initResult.Capabilities.DocumentSymbolProvider ||
		initResult.Capabilities.CompletionProvider == nil || !initResult.Capabilities.RenameProvider {
		t.Errorf("initialize result missing a rich-feature capability: %+v", initResult.Capabilities)
	}

	if msgs[1].Method != "textDocument/publishDiagnostics" {
		t.Fatalf("msgs[1].Method = %q, want textDocument/publishDiagnostics", msgs[1].Method)
	}

	var loc Location
	if err := json.Unmarshal(msgs[2].Result, &loc); err != nil {
		t.Fatalf("unmarshal definition result: %s", err)
	}
	if loc.Range.Start != (Position{Line: 0, Character: 7}) {
		t.Errorf("definition = %+v, want helper's declaration at 0:7", loc.Range.Start)
	}

	var refs []Location
	if err := json.Unmarshal(msgs[3].Result, &refs); err != nil {
		t.Fatalf("unmarshal references result: %s", err)
	}
	if len(refs) != 2 {
		t.Errorf("len(refs) = %d, want 2 (declaration + call site)", len(refs))
	}

	var syms []DocumentSymbol
	if err := json.Unmarshal(msgs[4].Result, &syms); err != nil {
		t.Fatalf("unmarshal documentSymbol result: %s", err)
	}
	if len(syms) != 1 || syms[0].Name != "helper" {
		t.Errorf("syms = %+v, want just helper", syms)
	}

	var items []CompletionItem
	if err := json.Unmarshal(msgs[5].Result, &items); err != nil {
		t.Fatalf("unmarshal completion result: %s", err)
	}
	if len(items) == 0 {
		t.Error("completion result is empty")
	}

	var edit WorkspaceEdit
	if err := json.Unmarshal(msgs[6].Result, &edit); err != nil {
		t.Fatalf("unmarshal rename result: %s", err)
	}
	if len(edit.Changes[uri]) != 2 {
		t.Errorf("len(edit.Changes[uri]) = %d, want 2", len(edit.Changes[uri]))
	}
}

// TestServerTypeDefinitionAliasesDefinition confirms
// textDocument/typeDefinition doesn't error out (the real bug report
// this aliasing fixes — Neovim's default <leader>D keymap sends this
// method, and a server that doesn't handle it at all gets "method not
// supported" back) and, since cRust has no separate type-declaration
// site to point at, returns exactly what textDocument/definition would.
func TestServerTypeDefinitionAliasesDefinition(t *testing.T) {
	src := "total = 1\ndeliver(total)\n"
	pos := map[string]any{"line": 1, "character": 9} // "total" inside deliver(...)

	var input bytes.Buffer
	input.Write(clientMessage(t, "textDocument/didOpen", nil, map[string]any{
		"textDocument": map[string]any{"uri": uri, "languageId": "crust", "version": 1, "text": src},
	}))
	input.Write(clientMessage(t, "textDocument/definition", 1, map[string]any{
		"textDocument": map[string]any{"uri": uri}, "position": pos,
	}))
	input.Write(clientMessage(t, "textDocument/typeDefinition", 2, map[string]any{
		"textDocument": map[string]any{"uri": uri}, "position": pos,
	}))
	input.Write(clientMessage(t, "exit", nil, nil))

	var output bytes.Buffer
	s := NewServer()
	if err := s.Run(&input, &output, io.Discard); err != nil {
		t.Fatalf("Run: %s", err)
	}
	msgs := decodeServerMessages(t, output.Bytes())
	if len(msgs) != 3 { // publishDiagnostics, definition, typeDefinition
		t.Fatalf("got %d messages, want 3: %+v", len(msgs), msgs)
	}

	var defResult, typeDefResult Location
	if err := json.Unmarshal(msgs[1].Result, &defResult); err != nil {
		t.Fatalf("unmarshal definition result: %s", err)
	}
	if err := json.Unmarshal(msgs[2].Result, &typeDefResult); err != nil {
		t.Fatalf("unmarshal typeDefinition result: %s", err)
	}
	if defResult != typeDefResult {
		t.Errorf("typeDefinition = %+v, want it to match definition = %+v", typeDefResult, defResult)
	}
}

// TestServerUnknownMethod confirms a request for an unhandled method
// gets a proper JSON-RPC method-not-found error response rather than
// being silently dropped or crashing the session.
func TestServerUnknownMethod(t *testing.T) {
	var input bytes.Buffer
	input.Write(clientMessage(t, "textDocument/codeAction", 1, map[string]any{}))
	input.Write(clientMessage(t, "exit", nil, nil))

	var output bytes.Buffer
	s := NewServer()
	if err := s.Run(&input, &output, io.Discard); err != nil {
		t.Fatalf("Run: %s", err)
	}

	msgs := decodeServerMessages(t, output.Bytes())
	if len(msgs) != 1 {
		t.Fatalf("got %d messages, want 1", len(msgs))
	}
	if msgs[0].Error == nil || msgs[0].Error.Code != errMethodNotFound {
		t.Errorf("response = %+v, want a MethodNotFound error", msgs[0])
	}
}

// TestServerDidCloseClearsDiagnostics confirms closing a document
// republishes an empty diagnostics list for it, rather than leaving a
// stale squiggle in the client for a buffer that's gone.
func TestServerDidCloseClearsDiagnostics(t *testing.T) {
	var input bytes.Buffer
	input.Write(clientMessage(t, "textDocument/didOpen", nil, map[string]any{
		"textDocument": map[string]any{
			"uri": "file:///bad.crust", "languageId": "crust", "version": 1, "text": "@\n",
		},
	}))
	input.Write(clientMessage(t, "textDocument/didClose", nil, map[string]any{
		"textDocument": map[string]any{"uri": "file:///bad.crust"},
	}))
	input.Write(clientMessage(t, "exit", nil, nil))

	var output bytes.Buffer
	s := NewServer()
	if err := s.Run(&input, &output, io.Discard); err != nil {
		t.Fatalf("Run: %s", err)
	}

	msgs := decodeServerMessages(t, output.Bytes())
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2", len(msgs))
	}
	var closeParams publishDiagnosticsParams
	if err := json.Unmarshal(msgs[1].Params, &closeParams); err != nil {
		t.Fatalf("unmarshal: %s", err)
	}
	if len(closeParams.Diagnostics) != 0 {
		t.Errorf("diagnostics after didClose = %+v, want none", closeParams.Diagnostics)
	}
}

// TestServerMalformedMessageRecovers confirms one garbled message
// doesn't end the session — the server should log it and keep serving
// whatever comes after, matching Run's documented behavior.
func TestServerMalformedMessageRecovers(t *testing.T) {
	var input bytes.Buffer
	body := []byte(`{not valid json`)
	fmt.Fprintf(&input, "Content-Length: %d\r\n\r\n", len(body))
	input.Write(body)
	input.Write(clientMessage(t, "exit", nil, nil))

	var output bytes.Buffer
	var logbuf bytes.Buffer
	s := NewServer()
	if err := s.Run(&input, &output, &logbuf); err != nil {
		t.Fatalf("Run: %s", err)
	}
	if logbuf.Len() == 0 {
		t.Error("expected a log line about the malformed message, got none")
	}
}
