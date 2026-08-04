package debugger

import "time"

// Turning a recording's raw durations/sizes into per-row and per-KPI
// numbers.
//
// Two facts shape this, borrowed directly from the design this package
// adapts (see recorder.go's package doc): durations *nest* (a step's
// own Dur already includes every frame it opened, so summing every
// recorded step's Dur would count the same nanoseconds once per level
// of nesting), and a value's *self* cost — what a row is actually
// responsible for, once its own nested frames are subtracted out — is
// the number worth ranking by, not the inclusive one (a `knead` loop
// at 98% total isn't a slow loop; it's a hundred laps of whatever is
// inside it).
//
// cRust's own fork from that design: rather than bucketing by call
// site (a fixed AST node — right for "which line is slow", but a
// recursive recipe would then show up as one row per call *site*, not
// one row for the recipe), KPIs bucket by frame *family* (recorder.go's
// family()) — every call to the same recipe, and every lap of the same
// loop, folds into one row, matching what "time/memory per function"
// actually asks for.

// NodeTiming is one row's cost, both ways of counting it.
type NodeTiming struct {
	Total time.Duration // including everything nested inside this row
	Self  time.Duration // Total minus this row's own frames' work
}

// KPI is one family's (a recipe's, or one loop's) running total across
// every call/lap it made, in the same self/total split as NodeTiming —
// Self is what answers "where did the time/memory actually go."
type KPI struct {
	Name      string
	Calls     int // frame instances: calls for a recipe, laps for a loop
	SelfTime  time.Duration
	TotalTime time.Duration
	SelfSize  int // summed value size (trace.SizeOf) of steps counted as this family's own
	Failed    int // steps under this family whose result was an Error
}

// TopLevelFamily is the KPI bucket for statements not inside any frame
// — top-level code, the common shape for a small AoC script that
// never defines a recipe at all.
const TopLevelFamily = "(top level)"

// Timing is the timing/KPI profile of a recording.
type Timing struct {
	overall time.Duration
	nodes   map[*TraceNode]NodeTiming
	kpis    map[string]*KPI
	order   []string // family names in first-seen order, for stable output
}

// Timing computes the recording's timing/KPI profile. Derived rather
// than accumulated during the run: the recorder is on the hot path of
// a traced run, and the tree already holds every duration/size once
// the run is over, so there's nothing to gain by computing this twice.
func (r *Recorder) Timing() *Timing {
	t := &Timing{
		nodes: make(map[*TraceNode]NodeTiming, r.steps),
		kpis:  map[string]*KPI{},
	}
	for _, n := range r.Roots() {
		t.overall += t.measure(n, TopLevelFamily)
	}
	return t
}

// measure computes one row's Total/Self and folds it into a KPI
// bucket, returning the row's inclusive duration. fam is the nearest
// enclosing frame's family (TopLevelFamily outside any frame), used to
// attribute a *step's* Self cost — a step belongs to whatever frame
// contains it, not the frame it might itself open. Entering a frame
// switches family to that frame's own for its subtree, and the frame
// itself becomes one Calls instance of that family, so a family's
// Calls count is "how many times this recipe ran / this loop lapped",
// never "how many statements it contains."
func (t *Timing) measure(n *TraceNode, fam string) time.Duration {
	childFam := fam
	if n.IsFrame() {
		childFam = family(n.Frame)
	}

	var nested time.Duration
	for _, c := range n.Children {
		nested += t.measure(c, childFam)
	}

	// A step's own duration already includes its frames; a frame isn't
	// timed itself, only what happened inside it.
	total := nested
	if !n.IsFrame() {
		total = n.Step.Dur
	}
	self := total - nested
	if self < 0 {
		// Clock granularity can leave children summing past their
		// parent by a few nanoseconds; zero is the honest floor.
		self = 0
	}
	t.nodes[n] = NodeTiming{Total: total, Self: self}

	switch {
	case n.IsFrame() && !n.Folded:
		t.kpiFor(childFam).addCall(total)
	case !n.IsFrame():
		t.kpiFor(fam).addStep(self, n.Step)
	}
	return total
}

// kpiFor returns fam's KPI bucket, creating it (in first-seen order,
// for stable ranked output) if this is the first row to touch it.
func (t *Timing) kpiFor(fam string) *KPI {
	k, ok := t.kpis[fam]
	if !ok {
		k = &KPI{Name: fam}
		t.kpis[fam] = k
		t.order = append(t.order, fam)
	}
	return k
}

// addCall folds one frame instance (one recipe call, or one loop lap)
// into the KPI: its own Calls count and inclusive TotalTime.
func (k *KPI) addCall(total time.Duration) {
	k.Calls++
	k.TotalTime += total
}

// addStep folds one step's already-self-only cost into the KPI: the
// SelfTime/SelfSize numbers a pie chart should actually be built from,
// since only Self figures sum to the whole run without double-counting
// nested calls.
func (k *KPI) addStep(self time.Duration, s *Step) {
	k.SelfTime += self
	if s.SizeOK {
		k.SelfSize += s.Size
	}
	if s.Failed() {
		k.Failed++
	}
}

// Of returns n's timing, or a zero NodeTiming if n wasn't measured
// (never happens for a row that came from this same Recorder's Roots()).
func (t *Timing) Of(n *TraceNode) NodeTiming {
	return t.nodes[n]
}

// Overall is the whole recording's inclusive duration — the sum of the
// top-level rows only, which is what makes every row's percentage of
// it add up to 100 rather than several times over.
func (t *Timing) Overall() time.Duration {
	return t.overall
}

// KPIs returns every family's totals, ranked by self time descending
// (ties broken by first-seen order) — "which function/loop is the
// work", the same question a profiler's hotspot list answers.
func (t *Timing) KPIs() []KPI {
	out := make([]KPI, 0, len(t.order))
	for _, name := range t.order {
		out = append(out, *t.kpis[name])
	}
	// Stable sort by self time descending; first-seen order (t.order)
	// is already the tiebreak since it's the input order.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].SelfTime > out[j-1].SelfTime; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
