package lsp

import "testing"

func TestComputeDiagnosticsIllegalToken(t *testing.T) {
	diags := computeDiagnostics("deliver(@)\n", "utf-16")
	if len(diags) != 1 {
		t.Fatalf("len(diags) = %d, want 1: %+v", len(diags), diags)
	}
	d := diags[0]
	if d.Severity != SeverityError {
		t.Errorf("Severity = %d, want %d", d.Severity, SeverityError)
	}
	if d.Range.Start != (Position{Line: 0, Character: 8}) {
		t.Errorf("Range.Start = %+v, want {0 8}", d.Range.Start)
	}
}

func TestComputeDiagnosticsParseError(t *testing.T) {
	// Unterminated block: no closing '}'.
	diags := computeDiagnostics("recipe foo() {\n", "utf-16")
	if len(diags) == 0 {
		t.Fatal("len(diags) = 0, want at least 1 parse error")
	}
}

func TestComputeDiagnosticsCleanSource(t *testing.T) {
	diags := computeDiagnostics(`deliver("hello")`+"\n", "utf-16")
	if len(diags) != 0 {
		t.Errorf("diags for clean source = %+v, want none", diags)
	}
}
