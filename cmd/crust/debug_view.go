package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Sintfoap/cRust/internal/debugger"
	"github.com/Sintfoap/cRust/internal/lexer"
	"github.com/Sintfoap/cRust/internal/object"
	"github.com/Sintfoap/cRust/internal/parser"
)

// debugView is everything the TUI (debug_tui.go) or the plain printer
// below needs — built once per `crust develop` run and handed to
// whichever of the two actually renders it.
type debugView struct {
	path string
	rec  *debugger.Recorder

	// Computed once on first use, so every caller gets the same answer
	// without needing to remember to ask for it, and so the TUI's Time/
	// Memory tabs don't recompute the whole timing pass on every redraw.
	timingOnce bool
	timingVal  *debugger.Timing

	// entryOnce/entryVal cache entryPoints() the same way — the Run
	// tab's selector (debug_run.go) asks on every render, and a fresh
	// parse per keystroke would be silly when the file hasn't changed
	// since this debugView was built.
	entryOnce bool
	entryVal  []string
}

// timing returns the recording's timing/KPI profile, computing it once.
func (v *debugView) timing() *debugger.Timing {
	if !v.timingOnce {
		v.timingOnce = true
		v.timingVal = v.rec.Timing()
	}
	return v.timingVal
}

// entryPoints returns the store/store_<name> entry points v.path
// declares (run.go's collectEntryPoints, "" for the bare `store`), for
// the Run tab's selector — computed once by re-reading and re-parsing
// the file independently of the current recording, since nothing else
// needs to keep an *ast.Program around. A parse failure returns no
// entry points rather than an error: an unparseable file already shows
// its own error on every other tab, so the Run tab just falls back to
// no selector instead of a second, redundant error message.
func (v *debugView) entryPoints() []string {
	if !v.entryOnce {
		v.entryOnce = true
		v.entryVal = scanEntryPoints(v.path)
	}
	return v.entryVal
}

// refreshEntryPoints discards any cached entryPoints() result, forcing
// the next call to rescan v.path from disk. Used when control hands
// back from an outside editor (debug_editor.go's handleNvimExit) —
// nvim could have added, removed, or renamed store/store_<name>
// recipes, and the Run tab's selector needs to reflect that as soon as
// the editor is exited, not only after a full retrace (which only
// happens when a save was actually detected).
func (v *debugView) refreshEntryPoints() {
	v.entryOnce = false
	v.entryVal = nil
}

// scanEntryPoints is entryPoints' actual work, split out so it needs
// no debugView receiver — handy for calling straight from a test.
func scanEntryPoints(path string) []string {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	l := lexer.New(string(src))
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil
	}
	return collectEntryPoints(program)
}

// header is the one-line description of the recording.
func (v *debugView) header() string {
	return fmt.Sprintf("%s — %s · %s total", v.path, v.rec.Summary(), v.timing().Overall().String())
}

// writePlain prints the recorded trace as an indented table, followed
// by the KPI ranking — the no-terminal form of the same information
// the TUI's two tabs show, and what makes `crust develop` scriptable
// (redirect to a file, grep it, assert on it in CI) rather than only
// usable interactively.
func (v *debugView) writePlain(w io.Writer) {
	t := v.timing()
	fmt.Fprintln(w, v.header())
	fmt.Fprintln(w, "% is a step's share of the run; self% excludes the work of any frame it opened (a call, a loop lap).")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s %s %6s %9s %7s\n", col("step", 44), col("out", 20), "size", "time", "self%")

	var walk func(nodes []*debugger.TraceNode, depth int)
	walk = func(nodes []*debugger.TraceNode, depth int) {
		for _, n := range nodes {
			indent := strings.Repeat("  ", depth)
			nt := t.Of(n)
			label, out, size := indent+n.Label(), "", ""
			if !n.IsFrame() {
				out = shortInspect(n.Step.Out)
				size = sizeText(n.Step)
			}
			fmt.Fprintf(w, "%s %s %6s %9s %7s\n",
				col(label, 44), col(out, 20), size, nt.Total.String(), selfPctText(nt, v.timing().Overall()))
			if !n.IsFrame() && n.Step.Failed() {
				fmt.Fprintf(w, "%serror: %s\n", indent+"  ", n.Step.Out.Inspect())
			}
			if laps, folded := n.Iterations(); folded {
				walk(n.Children[:1], depth+1)
				fmt.Fprintf(w, "%s… %d more iterations (--max-steps/full detail via the TUI)\n",
					strings.Repeat("  ", depth+1), laps-1)
				fmt.Fprintf(w, "%s// end %s\n", indent, closingLabel(n.Label()))
				continue
			}
			walk(n.Children, depth+1)
			if len(n.Children) > 0 {
				fmt.Fprintf(w, "%s// end %s\n", indent, closingLabel(n.Label()))
			}
		}
	}
	walk(v.rec.Roots(), 0)

	kpis := t.KPIs()
	if len(kpis) == 0 {
		return
	}
	fmt.Fprintln(w, "\nby self time:")
	fmt.Fprintf(w, "  %s %6s %9s %7s %8s\n", col("name", 30), "calls", "self", "%", "size")
	for _, k := range kpis {
		pct := "—"
		if v.timing().Overall() > 0 {
			pct = fmt.Sprintf("%.1f%%", 100*float64(k.SelfTime)/float64(v.timing().Overall()))
		}
		sizeCol := "—"
		if k.SelfSize > 0 {
			sizeCol = fmt.Sprintf("%d", k.SelfSize)
		}
		failedNote := ""
		if k.Failed > 0 {
			failedNote = fmt.Sprintf(" (%d failed)", k.Failed)
		}
		fmt.Fprintf(w, "  %s %6d %9s %7s %8s%s\n",
			col(k.Name, 30), k.Calls, k.SelfTime.String(), pct, sizeCol, failedNote)
	}
}

// closingLabel renders a node's label for its matching "// end" marker
// — the same text a "// end" line for, e.g., "knead r in (...) { …"
// would need is just "knead r in (...)", not another dangling "{ …"
// that has nothing left to continue into, and a loop header or call
// site can be long enough that the closing line deserves its own
// (shorter) truncation rather than reusing a table column width.
func closingLabel(label string) string {
	label = strings.TrimSuffix(label, " { …")
	label = strings.TrimSuffix(label, " …")
	return truncateRunes(label, 50)
}

// col renders a table cell w columns wide, counting runes so a
// multi-byte value doesn't shift the columns. Never clips: a label
// longer than its column pushes the rest of its own row right rather
// than losing characters, since this output is meant to be read and
// grepped — a truncated statement is worse than a ragged row.
func col(s string, w int) string {
	if n := len([]rune(s)); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

// selfPctText renders a row's self share, or an em dash when there's
// no denominator (a run too fast for the clock to resolve, or an
// overall time of zero).
func selfPctText(nt debugger.NodeTiming, overall time.Duration) string {
	if overall <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f%%", 100*float64(nt.Self)/float64(overall))
}

// sizeText renders a step's captured value size, or an em dash when
// the value has no meaningful size (a scalar — see trace.SizeOf).
func sizeText(s *debugger.Step) string {
	if !s.SizeOK {
		return "—"
	}
	return fmt.Sprintf("%d", s.Size)
}

// maxInspectRunes bounds a step's rendered value in the trace table —
// a single AoC input string or a large List shouldn't be able to blow
// out the table's column width.
const maxInspectRunes = 40

// shortInspect renders v's Inspect() form, truncated with an ellipsis
// past maxInspectRunes, or "nobox" for a nil-safe placeholder (nothing
// in a real recording should hand this a Go nil, but a step whose
// value the recorder didn't keep — past the size budget — shouldn't
// crash the printer over it either).
func shortInspect(v object.Object) string {
	if v == nil {
		return "nobox"
	}
	s := v.Inspect()
	runes := []rune(s)
	if len(runes) <= maxInspectRunes {
		return s
	}
	return string(runes[:maxInspectRunes-1]) + "…"
}
