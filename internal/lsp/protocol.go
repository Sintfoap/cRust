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

// contentChange is one entry of textDocument/didChange's
// contentChanges array. Range is absent for a full-document change
// (TextDocumentContentChangeEvent's whole-document-text variant) and
// present for an incremental one, where Text replaces exactly that
// range — see applyContentChange (sync.go). RangeLength is LSP's
// older, deprecated companion to Range (a UTF-16 code unit count
// covering the same span); never read here since Range alone is
// sufficient and is what every client capable of incremental sync
// already sends.
type contentChange struct {
	Range *Range `json:"range,omitempty"`
	Text  string `json:"text"`
}

type didChangeParams struct {
	TextDocument   versionedTextDocumentIdentifier `json:"textDocument"`
	ContentChanges []contentChange                 `json:"contentChanges"`
}

type didCloseParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

// textDocumentPositionParams is the common shape of every request that
// targets one cursor position in one document — hover, definition,
// references, rename, completion.
type textDocumentPositionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type hoverParams = textDocumentPositionParams

type markupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type hoverResult struct {
	Contents markupContent `json:"contents"`
}

// Location is an LSP Location: a Range within a specific document —
// what textDocument/definition and textDocument/references respond
// with.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

type referenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

type referenceParams struct {
	textDocumentPositionParams
	Context referenceContext `json:"context"`
}

type documentSymbolParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

// formattingParams is DocumentFormattingParams — the real LSP shape
// also carries an "options" field (tab size, spaces-vs-tabs, ...), left
// out here since internal/format is fully opinionated and takes no
// configuration, the same "one canonical style, no knobs" stance
// gofmt takes.
type formattingParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

// SymbolKind values this package uses (LSP SymbolKind enum) — only
// Function, since recipes are the only thing documentSymbol reports.
const symbolKindFunction = 12

// DocumentSymbol is one entry in a textDocument/documentSymbol
// response. Range and SelectionRange are the same span here (the
// recipe name's token) rather than the whole declaration — internal/ast
// doesn't carry a block's closing-brace position, so there's no cheap
// way to report the full body span; editors still get correct
// outline/go-to-symbol behavior from the name span alone.
type DocumentSymbol struct {
	Name           string `json:"name"`
	Kind           int    `json:"kind"`
	Range          Range  `json:"range"`
	SelectionRange Range  `json:"selectionRange"`
}

type renameParams struct {
	textDocumentPositionParams
	NewName string `json:"newName"`
}

// TextEdit is one LSP TextEdit: replace Range's text with NewText.
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// WorkspaceEdit is what textDocument/rename responds with — Changes
// maps a document URI to the edits to apply there. Every rename this
// package performs stays within one document, so Changes never has
// more than one key.
type WorkspaceEdit struct {
	Changes map[string][]TextEdit `json:"changes,omitempty"`
}

type completionParams = textDocumentPositionParams

// CompletionItemKind values this package uses (LSP CompletionItemKind
// enum).
const (
	completionKindFunction = 3
	completionKindVariable = 6
	completionKindKeyword  = 14
)

// CompletionItem is one suggestion in a textDocument/completion
// response.
type CompletionItem struct {
	Label  string `json:"label"`
	Kind   int    `json:"kind,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// completionOptions is LSP's CompletionOptions — completionProvider
// must be this shape, not a bare boolean, per spec. Empty: this
// package doesn't support completion-item resolve or trigger
// characters beyond the client's own defaults.
type completionOptions struct{}

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

// codeActionContext carries the diagnostics VS Code (and other
// clients) already know overlap the requested range — unused here,
// since the one code action this package offers (formatDocument, not
// diagnostic-specific) doesn't need to know which diagnostics
// triggered the request.
type codeActionContext struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type codeActionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Range        Range                  `json:"range"`
	Context      codeActionContext      `json:"context"`
}

// CodeAction is one LSP CodeAction — Edit is filled in directly rather
// than a Command, since every action this package offers is a pure
// text edit with nothing further for the server to execute once the
// client applies it.
type CodeAction struct {
	Title string         `json:"title"`
	Kind  string         `json:"kind,omitempty"`
	Edit  *WorkspaceEdit `json:"edit,omitempty"`
}

// CodeActionKind values this package uses (LSP CodeActionKind enum —
// a namespaced string, not an int, per spec).
const codeActionKindSourceFixAll = "source.fixAll"

type clientCapabilities struct {
	General *generalClientCapabilities `json:"general,omitempty"`
}

type initializeParams struct {
	Capabilities clientCapabilities `json:"capabilities"`
}

type serverCapabilities struct {
	TextDocumentSync           int                `json:"textDocumentSync"`
	HoverProvider              bool               `json:"hoverProvider"`
	PositionEncoding           string             `json:"positionEncoding"`
	DefinitionProvider         bool               `json:"definitionProvider"`
	TypeDefinitionProvider     bool               `json:"typeDefinitionProvider"`
	ReferencesProvider         bool               `json:"referencesProvider"`
	DocumentSymbolProvider     bool               `json:"documentSymbolProvider"`
	CompletionProvider         *completionOptions `json:"completionProvider,omitempty"`
	RenameProvider             bool               `json:"renameProvider"`
	DocumentFormattingProvider bool               `json:"documentFormattingProvider"`
	CodeActionProvider         bool               `json:"codeActionProvider"`
}

type initializeResult struct {
	Capabilities serverCapabilities `json:"capabilities"`
}

// textDocumentSyncKindIncremental is LSP's TextDocumentSyncKind.
// Incremental: didChange notifications carry a Range plus the text
// that replaces it, not the whole document — cheaper over the wire for
// a client editing a long file one keystroke at a time, since a
// keystroke's own contentChanges entry is a few bytes instead of the
// entire buffer. applyContentChange (sync.go) does the actual
// Range-to-byte-offset splice; a change with no Range (a full-document
// replace) is still accepted the same way full sync always was, since
// nothing in the spec requires every client to honor the server's
// preferred sync kind on every single edit.
const textDocumentSyncKindIncremental = 2
