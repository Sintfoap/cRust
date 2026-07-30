package ast

import (
	"testing"

	"github.com/Sintfoap/cRust/internal/token"
)

func TestIdentifier(t *testing.T) {
	id := &Identifier{Token: token.Token{Type: token.IDENT, Literal: "count"}, Value: "count"}

	if got := id.TokenLiteral(); got != "count" {
		t.Errorf("TokenLiteral() = %q, want %q", got, "count")
	}
	if got := id.String(); got != "count" {
		t.Errorf("String() = %q, want %q", got, "count")
	}

	// Compile-time interface satisfaction.
	var _ Expression = id
}
