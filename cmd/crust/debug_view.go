package main

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Sintfoap/cRust/internal/debugger"
	"github.com/Sintfoap/cRust/internal/object"
)

// debugView is everything the TUI (debug_tui.go) or the plain printer
// below needs — built once per `crust debug` run and handed to
// whichever of the two actually renders it.
type debugView struct {
	path string
	rec  *debugger.Recorder

	// Computed once on first use, so every caller gets the same answer
	// without needing to remember to ask for it, and so the TUI's KPI
	// tab doesn't recompute the whole timing pass on every redraw.
	timingOnce bool
	timingVal  *debugger.Timing
}

// timing returns the recording's timing/KPI profile, computing it once.
func (v *debugView) timing() *debugger.Timing {
	if !v.timingOnce {
		v.timingOnce = true
		v.timingVal = v.rec.Timing()
	}
	return v.timingVal
}

// header is the one-line description of the recording.
func (v *debugView) header() string {
	return fmt.Sprintf("%s — %s · %s total", v.path, v.rec.Summary(), v.timing().Overall().String())
}

// writePlain prints the recorded trace as an indented table, followed
// by the KPI ranking — the no-terminal form of the same information
// the TUI's two tabs show, and what makes `crust debug` scriptable
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
				continue
			}
			walk(n.Children, depth+1)
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
