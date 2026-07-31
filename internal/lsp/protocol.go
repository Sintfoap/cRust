package lsp

import "encoding/json"

// rpcMessage is the wire shape for every JSON-RPC 2.0 message this
// package sends or receives — request, response, and notification all
// share one Go type, distinguished by which fields are present
// (Method+ID = request, Method alone = notification, ID+Result/Error =
// response). Params/Result stay as json.RawMessage so dispatch can
// pick the concrete type to decode into per Method, and ID stays raw
// so a response can echo back whatever shape (number or string) the
// request used without a lossy round-trip through Go's one JSON number
// type.
type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSON-RPC 2.0 standard error codes (the LSP spec's own error codes,
// -32899..-32800, aren't needed by anything this package implements).
const (
	errParseError     = -32700
	errMethodNotFound = -32601
	errInvalidParams  = -32602
	errInternalError  = -32603
)

// Position is an LSP Position: 0-indexed line, and a character offset
// whose *unit* depends on the negotiated position encoding (utf-8
// bytes, utf-16 code units, or utf-32 code points — see position.go).
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range is an LSP Range: [Start, End), both Positions.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type textDocumentIdentifier struct {
	URI string `json:"uri"`
}

type versionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type textDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type didOpenParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type contentChange struct {
	Text string `json:"text"`
}

type didChangeParams struct {
	TextDocument   versionedTextDocumentIdentifier `json:"textDocument"`
	ContentChanges []contentChange                 `json:"contentChanges"`
}

type didCloseParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type hoverParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type markupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type hoverResult struct {
	Contents markupContent `json:"contents"`
}

// Diagnostic severities (LSP DiagnosticSeverity). Only SeverityError is
// used today — lex/parse failures are unambiguous errors, not
// warnings/hints.
const (
	SeverityError   = 1
	SeverityWarning = 2
	SeverityInfo    = 3
	SeverityHint    = 4
)

// Diagnostic is one LSP Diagnostic.
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity,omitempty"`
	Source   string `json:"source,omitempty"`
	Message  string `json:"message"`
}

type publishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type generalClientCapabilities struct {
	PositionEncodings []string `json:"positionEncodings,omitempty"`
}

type clientCapabilities struct {
	General *generalClientCapabilities `json:"general,omitempty"`
}

type initializeParams struct {
	Capabilities clientCapabilities `json:"capabilities"`
}

type serverCapabilities struct {
	TextDocumentSync int    `json:"textDocumentSync"`
	HoverProvider    bool   `json:"hoverProvider"`
	PositionEncoding string `json:"positionEncoding"`
}

type initializeResult struct {
	Capabilities serverCapabilities `json:"capabilities"`
}

// textDocumentSyncKindFull is LSP's TextDocumentSyncKind.Full: every
// didChange notification carries the whole new document text, not an
// incremental edit. The simplest correct choice for a language this
// small — no incremental-sync bookkeeping to get wrong.
const textDocumentSyncKindFull = 1
