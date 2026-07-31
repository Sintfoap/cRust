// Package lsp implements a Language Server Protocol server for cRust:
// hover and diagnostics, speaking JSON-RPC 2.0 over stdio. Hand-rolled
// against the spec rather than built on a third-party LSP library, to
// keep the project's zero-Go-dependency policy intact (see
// docs/ARCHITECTURE.md — this is also what keeps the Nix flake's
// `vendorHash = null` valid). `crust lsp` (cmd/crust/lsp.go) is the
// only thing that constructs a Server.
package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// Server is a cRust language server. Each Run call is one stdio
// session: one client, one lifecycle (initialize..shutdown/exit), one
// set of open documents. Not safe for concurrent use — LSP's stdio
// transport is inherently single-connection, so nothing here needs a
// mutex.
type Server struct {
	out      io.Writer
	docs     map[string]string
	encoding string // "utf-8", "utf-16", or "utf-32" — see negotiateEncoding
}

// NewServer returns a Server ready for Run.
func NewServer() *Server {
	return &Server{
		docs:     make(map[string]string),
		encoding: "utf-16", // LSP's documented default absent negotiation
	}
}

// Run reads JSON-RPC messages from in and writes responses/
// notifications to out until in reaches EOF or an "exit" notification
// arrives. logw receives one line per malformed message it recovers
// from (pass io.Discard to silence it) — never in itself a fatal
// error, since a language server should keep serving the rest of the
// session after one bad message.
func (s *Server) Run(in io.Reader, out io.Writer, logw io.Writer) error {
	s.out = out
	br := bufio.NewReader(in)
	for {
		body, err := readMessage(br)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		var msg rpcMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			fmt.Fprintf(logw, "lsp: malformed message: %s\n", err)
			continue
		}

		if s.handle(msg, logw) {
			return nil
		}
	}
}

// handle dispatches one message and reports whether the session should
// end (true only for "exit").
func (s *Server) handle(msg rpcMessage, logw io.Writer) bool {
	switch msg.Method {
	case "initialize":
		s.handleInitialize(msg)
	case "initialized":
		// No response expected; nothing to do on our side.
	case "shutdown":
		s.respondResult(msg.ID, nil)
	case "exit":
		return true
	case "textDocument/didOpen":
		s.handleDidOpen(msg, logw)
	case "textDocument/didChange":
		s.handleDidChange(msg, logw)
	case "textDocument/didClose":
		s.handleDidClose(msg, logw)
	case "textDocument/hover":
		s.handleHover(msg, logw)
	default:
		if len(msg.ID) > 0 {
			s.respondError(msg.ID, errMethodNotFound, fmt.Sprintf("method not found: %s", msg.Method))
		}
	}
	return false
}

func (s *Server) respondResult(id json.RawMessage, result any) {
	raw, err := json.Marshal(result)
	if err != nil {
		s.respondError(id, errInternalError, err.Error())
		return
	}
	writeMessage(s.out, rpcMessage{JSONRPC: "2.0", ID: id, Result: raw})
}

func (s *Server) respondError(id json.RawMessage, code int, message string) {
	writeMessage(s.out, rpcMessage{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: message}})
}

func (s *Server) notify(method string, params any) {
	raw, err := json.Marshal(params)
	if err != nil {
		return
	}
	writeMessage(s.out, rpcMessage{JSONRPC: "2.0", Method: method, Params: raw})
}

func (s *Server) handleInitialize(msg rpcMessage) {
	var params initializeParams
	if len(msg.Params) > 0 {
		_ = json.Unmarshal(msg.Params, &params)
	}
	s.encoding = negotiateEncoding(params.Capabilities.General)

	s.respondResult(msg.ID, initializeResult{
		Capabilities: serverCapabilities{
			TextDocumentSync: textDocumentSyncKindFull,
			HoverProvider:    true,
			PositionEncoding: s.encoding,
		},
	})
}

func (s *Server) handleDidOpen(msg rpcMessage, logw io.Writer) {
	var params didOpenParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		fmt.Fprintf(logw, "lsp: bad didOpen params: %s\n", err)
		return
	}
	s.docs[params.TextDocument.URI] = params.TextDocument.Text
	s.publishDiagnostics(params.TextDocument.URI)
}

func (s *Server) handleDidChange(msg rpcMessage, logw io.Writer) {
	var params didChangeParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		fmt.Fprintf(logw, "lsp: bad didChange params: %s\n", err)
		return
	}
	if len(params.ContentChanges) == 0 {
		return
	}
	// Full sync (TextDocumentSyncKind.Full, advertised in initialize) —
	// the last content change's Text is the whole new document.
	s.docs[params.TextDocument.URI] = params.ContentChanges[len(params.ContentChanges)-1].Text
	s.publishDiagnostics(params.TextDocument.URI)
}

func (s *Server) handleDidClose(msg rpcMessage, logw io.Writer) {
	var params didCloseParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		fmt.Fprintf(logw, "lsp: bad didClose params: %s\n", err)
		return
	}
	delete(s.docs, params.TextDocument.URI)
	// Clear diagnostics for a closed document — a stale squiggle on a
	// buffer that no longer exists client-side would be a bug, not a
	// feature.
	s.notify("textDocument/publishDiagnostics", publishDiagnosticsParams{
		URI:         params.TextDocument.URI,
		Diagnostics: []Diagnostic{},
	})
}

func (s *Server) handleHover(msg rpcMessage, logw io.Writer) {
	var params hoverParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		s.respondError(msg.ID, errInvalidParams, err.Error())
		return
	}
	text, ok := s.docs[params.TextDocument.URI]
	if !ok {
		s.respondResult(msg.ID, nil)
		return
	}
	s.respondResult(msg.ID, hoverAt(text, params.Position, s.encoding))
}

func (s *Server) publishDiagnostics(uri string) {
	diags := computeDiagnostics(s.docs[uri], s.encoding)
	s.notify("textDocument/publishDiagnostics", publishDiagnosticsParams{URI: uri, Diagnostics: diags})
}
