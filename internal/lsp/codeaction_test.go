package lsp

import "testing"

func TestCodeActionsForUnformattedDocumentOffersFormat(t *testing.T) {
	actions := codeActionsFor("x=1\ndeliver(x)\n", "file:///day01.crust", "utf-16")
	if len(actions) != 1 {
		t.Fatalf("got %d actions, want 1: %+v", len(actions), actions)
	}
	a := actions[0]
	if a.Title != "Format document" {
		t.Errorf("Title = %q, want %q", a.Title, "Format document")
	}
	if a.Kind != codeActionKindSourceFixAll {
		t.Errorf("Kind = %q, want %q", a.Kind, codeActionKindSourceFixAll)
	}
	if a.Edit == nil {
		t.Fatal("Edit = nil, want a WorkspaceEdit")
	}
	edits, ok := a.Edit.Changes["file:///day01.crust"]
	if !ok || len(edits) == 0 {
		t.Errorf("Edit.Changes = %+v, want an entry for the document URI", a.Edit.Changes)
	}
}

func TestCodeActionsForAlreadyFormattedDocumentIsEmpty(t *testing.T) {
	actions := codeActionsFor("x = 1\ndeliver(x)\n", "file:///day01.crust", "utf-16")
	if len(actions) != 0 {
		t.Errorf("got %d actions, want 0 for an already-canonical document: %+v", len(actions), actions)
	}
	if actions == nil {
		t.Error("got nil, want a non-nil empty slice")
	}
}

func TestCodeActionsForUnparseableDocumentIsEmpty(t *testing.T) {
	actions := codeActionsFor("x = (\n", "file:///day01.crust", "utf-16")
	if len(actions) != 0 {
		t.Errorf("got %d actions, want 0 for an unparseable document: %+v", len(actions), actions)
	}
}
