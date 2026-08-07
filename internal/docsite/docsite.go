// Package docsite is the browsable documentation website `crust bake
// documentation` (cmd/crust) serves over local HTTP. The templates
// (embedded at build time via go:embed, so this works from any built
// binary with no source tree present) are static, but the two
// reference pages — Keywords and Standard Library — are rendered from
// internal/lsp's own KeywordDocs/BuiltinDocs accessors rather than a
// second, hand-maintained copy of the language's vocabulary. Those are
// already kept honest against internal/builtins' real registrations by
// internal/lsp's own TestBuiltinDocsCoversEveryRealBuiltin, so this
// site inherits that guarantee for free: whatever `crust lsp` would
// hover for a name is exactly what this site prints for it, and a
// forgotten doc entry fails a test in internal/lsp long before it
// could ever go stale here.
package docsite

import (
	"embed"
	"html/template"
	"net/http"
	"sort"
	"strings"

	"github.com/Sintfoap/cRust/internal/lsp"
)

//go:embed templates/*.html.tmpl
var templatesFS embed.FS

// pageHeader is the data every page template needs regardless of its
// own content: which sidebar link to highlight and what to put in
// <title>. Every page-specific data struct embeds it so the shared
// layout template (which only ever sees .Active/.Title) and a page's
// own content block (which sees whatever else that struct adds) can
// read the exact same value passed to Execute.
type pageHeader struct {
	Active string
	Title  string
}

// docEntry is one row of the Keywords or Standard Library reference
// table. Doc is template.HTML, not string, so renderInlineDoc's output
// (built entirely from this binary's own compile-time string literals
// in internal/lsp, never from anything a request could influence) is
// trusted straight through instead of being re-escaped into visible
// angle brackets.
type docEntry struct {
	Name string
	Doc  template.HTML
}

type keywordsPage struct {
	pageHeader
	Keywords []docEntry
}

type builtinsPage struct {
	pageHeader
	Builtins []docEntry
}

// pageNames lists every page this site serves, one .html.tmpl file
// each, all built on the shared templates/layout.html.tmpl.
var pageNames = []string{"home", "getting-started", "keywords", "builtins", "grammar", "examples"}

// pages holds one parsed *template.Template per page name, each built
// by parsing layout.html.tmpl together with just that one page's file
// — combining every page into a single parse (e.g. via ParseGlob)
// would fail outright, since every page's file defines a same-named
// {{define "content"}} block and html/template errors on redefining a
// template name within one parse. Parsing per-page instead sidesteps
// that entirely: each combination only ever sees one "content"
// definition. Built once at package init (Must, not a returned error)
// since the templates are embedded, fixed at compile time, and their
// own tests (TestAllPagesRender) already catch a syntax mistake in CI
// long before any binary built from a broken commit could reach this
// line at runtime.
var pages = func() map[string]*template.Template {
	out := make(map[string]*template.Template, len(pageNames))
	for _, name := range pageNames {
		out[name] = template.Must(template.ParseFS(templatesFS, "templates/layout.html.tmpl", "templates/"+name+".html.tmpl"))
	}
	return out
}()

// renderInlineDoc converts the light backtick-code-span markup
// internal/lsp's hover doc strings use (e.g. a doc string containing
// backtick-quoted `recipe`) into HTML, escaping everything else. Not a general
// Markdown renderer — cRust's own hover docs only ever use backtick
// spans, nothing else Markdown offers (no lists, links, emphasis), so
// there's nothing more this needs to handle. Segments alternate
// plain/code starting with plain (a leading backtick would make index
// 0 the empty string before the first code span, which still lands on
// the correct side of the alternation).
func renderInlineDoc(s string) template.HTML {
	var b strings.Builder
	for i, seg := range strings.Split(s, "`") {
		esc := template.HTMLEscapeString(seg)
		if i%2 == 1 {
			b.WriteString("<code>")
			b.WriteString(esc)
			b.WriteString("</code>")
		} else {
			b.WriteString(esc)
		}
	}
	return template.HTML(b.String())
}

// sortedEntries turns a name->doc map into name-sorted docEntry rows —
// map iteration order is randomized per Go's own spec, and a reference
// table that reshuffled itself on every page load would be unreadable.
func sortedEntries(docs map[string]string) []docEntry {
	names := make([]string, 0, len(docs))
	for name := range docs {
		names = append(names, name)
	}
	sort.Strings(names)
	entries := make([]docEntry, len(names))
	for i, name := range names {
		entries[i] = docEntry{Name: name, Doc: renderInlineDoc(docs[name])}
	}
	return entries
}

// Handler serves the documentation site: "/" is the home page, every
// other route is one more static or data-driven page, and anything
// else is a 404 — a bare mux.Handle("/", ...) alone would silently
// treat every unmatched path as the home page instead, since "/" is
// Go's own catch-all pattern.
func Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		renderPage(w, "home", pageHeader{Active: "home", Title: "the docs, baked fresh"})
	})
	mux.HandleFunc("/getting-started", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, "getting-started", pageHeader{Active: "getting-started", Title: "Getting Started"})
	})
	mux.HandleFunc("/keywords", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, "keywords", keywordsPage{
			pageHeader: pageHeader{Active: "keywords", Title: "Keywords"},
			Keywords:   sortedEntries(lsp.KeywordDocs()),
		})
	})
	mux.HandleFunc("/builtins", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, "builtins", builtinsPage{
			pageHeader: pageHeader{Active: "builtins", Title: "Standard Library"},
			Builtins:   sortedEntries(lsp.BuiltinDocs()),
		})
	})
	mux.HandleFunc("/grammar", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, "grammar", pageHeader{Active: "grammar", Title: "Syntax Reference"})
	})
	mux.HandleFunc("/examples", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, "examples", pageHeader{Active: "examples", Title: "Examples"})
	})

	return mux
}

// renderPage executes name's template with data. A render failure
// (only reachable if a template itself is malformed — impossible for
// the embedded set once TestAllPagesRender passes, but Execute can
// still fail this way in principle) reports 500 rather than a half-
// written page.
func renderPage(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := pages[name].Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
