package docsite

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sintfoap/cRust/internal/lsp"
)

// TestAllPagesRender confirms every page name in pageNames has a
// template that parses and executes without error against real data —
// the same guard a syntax mistake in any templates/*.html.tmpl file
// would need to be caught by, since template.Must at package init
// already panics on a parse error; this instead exercises Execute,
// which a parse error alone can't catch (an undefined template
// variable, for instance, only fails at execution).
func TestAllPagesRender(t *testing.T) {
	h := Handler()
	routes := map[string]string{
		"home":            "/",
		"getting-started": "/getting-started",
		"keywords":        "/keywords",
		"builtins":        "/builtins",
		"grammar":         "/grammar",
		"examples":        "/examples",
	}
	for name, path := range routes {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s = %d, want 200; body: %s", path, rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			if !strings.Contains(body, "<html") {
				t.Errorf("GET %s: response doesn't look like HTML: %q", path, body)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
				t.Errorf("GET %s: Content-Type = %q, want text/html", path, ct)
			}
		})
	}
}

// TestUnknownPathIs404 confirms the "/" handler's explicit path check
// is actually doing something -- without it, ServeMux's own catch-all
// semantics for "/" would silently serve the home page for any
// unmatched path instead.
func TestUnknownPathIs404(t *testing.T) {
	h := Handler()
	req := httptest.NewRequest(http.MethodGet, "/no-such-page", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /no-such-page = %d, want 404", rec.Code)
	}
}

// TestKeywordsPageListsRealKeywords confirms the Keywords page is
// actually data-driven from lsp.KeywordDocs, not static placeholder
// text -- a handful of keywords spanning different parts of the
// vocabulary (control flow, boolean literals, logical operators) all
// need to show up.
func TestKeywordsPageListsRealKeywords(t *testing.T) {
	h := Handler()
	req := httptest.NewRequest(http.MethodGet, "/keywords", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	for _, kw := range []string{"recipe", "knead", "stuffed", "hold"} {
		if !strings.Contains(body, kw) {
			t.Errorf("GET /keywords: body missing keyword %q", kw)
		}
	}
}

// TestBuiltinsPageListsRealBuiltins confirms the Standard Library page
// reads live from lsp.BuiltinDocs -- including the stdlib additions
// from this same session (abs/pow/sqrt/gcd/lcm/replace/upper/lower),
// which is the entire point of sourcing this page from the real table
// instead of hand-written prose: it can't lag behind what's actually
// registered.
func TestBuiltinsPageListsRealBuiltins(t *testing.T) {
	h := Handler()
	req := httptest.NewRequest(http.MethodGet, "/builtins", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	for _, name := range []string{"deliver", "abs", "pow", "sqrt", "gcd", "lcm", "replace", "upper", "lower", "contains"} {
		if !strings.Contains(body, name) {
			t.Errorf("GET /builtins: body missing builtin %q", name)
		}
	}
}

// TestBuiltinsPageMatchesLiveTableCount confirms the page renders
// exactly as many rows as lsp.BuiltinDocs() has entries -- a stray
// filter or an off-by-one in sortedEntries would otherwise silently
// drop or duplicate a row without any of the substring checks above
// catching it.
func TestBuiltinsPageMatchesLiveTableCount(t *testing.T) {
	entries := sortedEntries(lsp.BuiltinDocs())
	want := len(lsp.BuiltinDocs())
	if len(entries) != want {
		t.Fatalf("sortedEntries(BuiltinDocs()) has %d entries, want %d", len(entries), want)
	}
}

// TestSortedEntriesIsAlphabetical confirms the reference tables render
// in a stable, readable order rather than Go's randomized map
// iteration order leaking straight into the page.
func TestSortedEntriesIsAlphabetical(t *testing.T) {
	entries := sortedEntries(map[string]string{"zebra": "z", "apple": "a", "mango": "m"})
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}
	if entries[0].Name != "apple" || entries[1].Name != "mango" || entries[2].Name != "zebra" {
		t.Errorf("order = [%s, %s, %s], want alphabetical", entries[0].Name, entries[1].Name, entries[2].Name)
	}
}

func TestRenderInlineDocWrapsBacktickSpans(t *testing.T) {
	got := renderInlineDoc("`recipe` — function definition")
	want := "<code>recipe</code> — function definition"
	if string(got) != want {
		t.Errorf("renderInlineDoc() = %q, want %q", got, want)
	}
}

// TestRenderInlineDocEscapesHTML confirms text outside (and inside)
// backtick spans is HTML-escaped -- necessary defense in depth even
// though every real caller passes a compile-time string literal from
// internal/lsp, never anything a request could influence.
func TestRenderInlineDocEscapesHTML(t *testing.T) {
	got := renderInlineDoc("a < b `x < y`")
	if strings.Contains(string(got), "<") && !strings.Contains(string(got), "&lt;") {
		t.Errorf("renderInlineDoc(%q) = %q, want < escaped", "a < b `x < y`", got)
	}
	if !strings.Contains(string(got), "&lt; b") {
		t.Errorf("renderInlineDoc() = %q, want the plain-text segment escaped", got)
	}
	if !strings.Contains(string(got), "<code>x &lt; y</code>") {
		t.Errorf("renderInlineDoc() = %q, want the code-span segment escaped inside <code>", got)
	}
}

func TestRenderInlineDocNoBackticksIsPlainEscapedText(t *testing.T) {
	got := renderInlineDoc("plain text, no code spans")
	if string(got) != "plain text, no code spans" {
		t.Errorf("renderInlineDoc() = %q, want unchanged plain text", got)
	}
}
