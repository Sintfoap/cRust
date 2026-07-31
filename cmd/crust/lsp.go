package main

import (
	"io"

	"github.com/Sintfoap/cRust/internal/lsp"
)

// runLSP starts a cRust language server on stdin/stdout, per the LSP
// spec's stdio transport — the mode every editor client (Neovim's
// vim.lsp.start included) expects when it launches a server as a
// subprocess. stderr is used as the server's log stream: LSP forbids
// writing anything but framed protocol messages to stdout, so
// diagnostics about malformed client messages go to stderr instead,
// where editors typically surface a language server's own log output.
func runLSP(stdin io.Reader, stdout, stderr io.Writer) int {
	s := lsp.NewServer()
	if err := s.Run(stdin, stdout, stderr); err != nil {
		return 1
	}
	return 0
}
