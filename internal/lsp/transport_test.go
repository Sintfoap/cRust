package lsp

import (
	"bufio"
	"bytes"
	"io"
	"strings"
	"testing"
)

// TestTransportRoundTrip exercises writeMessage/readMessage against a
// real byte stream (not a hand-mocked one) — write a message, then
// read it back off the exact bytes writeMessage produced, the way a
// real client/server pair would over stdio.
func TestTransportRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	msg := rpcMessage{JSONRPC: "2.0", Method: "textDocument/hover"}
	if err := writeMessage(&buf, msg); err != nil {
		t.Fatalf("writeMessage: %s", err)
	}

	got, err := readMessage(bufio.NewReader(&buf))
	if err != nil {
		t.Fatalf("readMessage: %s", err)
	}
	want := `{"jsonrpc":"2.0","method":"textDocument/hover"}`
	if string(got) != want {
		t.Errorf("readMessage body = %s, want %s", got, want)
	}
}

// TestTransportMultipleMessages confirms readMessage correctly frames
// consecutive messages back to back on one stream — the normal case
// for a real session, where many notifications/requests arrive on the
// same connection.
func TestTransportMultipleMessages(t *testing.T) {
	var buf bytes.Buffer
	writeMessage(&buf, rpcMessage{JSONRPC: "2.0", Method: "one"})
	writeMessage(&buf, rpcMessage{JSONRPC: "2.0", Method: "two"})

	r := bufio.NewReader(&buf)
	first, err := readMessage(r)
	if err != nil {
		t.Fatalf("first readMessage: %s", err)
	}
	second, err := readMessage(r)
	if err != nil {
		t.Fatalf("second readMessage: %s", err)
	}
	if string(first) != `{"jsonrpc":"2.0","method":"one"}` {
		t.Errorf("first = %s", first)
	}
	if string(second) != `{"jsonrpc":"2.0","method":"two"}` {
		t.Errorf("second = %s", second)
	}

	if _, err := readMessage(r); err != io.EOF {
		t.Errorf("readMessage at end = %v, want io.EOF", err)
	}
}

func TestTransportMissingContentLength(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("Content-Type: application/json\r\n\r\n{}"))
	if _, err := readMessage(r); err == nil {
		t.Errorf("readMessage with no Content-Length: got nil error, want one")
	}
}
