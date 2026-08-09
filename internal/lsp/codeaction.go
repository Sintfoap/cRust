package lsp

// codeActionsFor returns the code actions available for text at uri —
// today, just "Format document" when text isn't already canonically
// formatted, reusing formatDocument's own idempotency check (the same
// one `crust fmt -w` and format-on-save already rely on) so this never
// offers a no-op edit. An empty (non-nil) slice, not nil, when there's
// nothing to offer — an unparseable document (formatDocument returns
// nil) or one that's already canonical — so a client's lightbulb menu
// shows "no actions available" rather than treating a nil result as an
// error.
func codeActionsFor(text string, uri string, encoding string) []CodeAction {
	edits := formatDocument(text, encoding)
	if len(edits) == 0 {
		return []CodeAction{}
	}
	return []CodeAction{{
		Title: "Format document",
		Kind:  codeActionKindSourceFixAll,
		Edit: &WorkspaceEdit{
			Changes: map[string][]TextEdit{uri: edits},
		},
	}}
}
