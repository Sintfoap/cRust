package debugger

import (
	"testing"
)

func kpiByName(kpis []KPI, name string) (KPI, bool) {
	for _, k := range kpis {
		if k.Name == name {
			return k, true
		}
	}
	return KPI{}, false
}

func TestTimingKPIsGroupTopLevelStatements(t *testing.T) {
	rec := record(t, "x = 1\ny = 2\n", 0)
	timing := rec.Timing()
	kpis := timing.KPIs()
	if len(kpis) != 1 {
		t.Fatalf("got %d KPI buckets, want 1 (top level only); kpis = %+v", len(kpis), kpis)
	}
	if kpis[0].Name != TopLevelFamily {
		t.Errorf("bucket name = %q, want %q", kpis[0].Name, TopLevelFamily)
	}
}

func TestTimingKPIsGroupAllCallsToOneRecipeTogether(t *testing.T) {
	rec := record(t, `
recipe double(x) {
    serve x * 2
}
a = double(1)
b = double(2)
c = double(3)
`, 0)
	timing := rec.Timing()
	kpis := timing.KPIs()
	k, ok := kpiByName(kpis, "double(...)")
	if !ok {
		t.Fatalf("no KPI bucket for double(...); kpis = %+v", kpis)
	}
	if k.Calls != 3 {
		t.Errorf("Calls = %d, want 3 (one per call, not one per statement)", k.Calls)
	}
}

func TestTimingKPIsGroupRecursiveCallsIntoOneBucket(t *testing.T) {
	rec := record(t, `
recipe fact(n) {
    order (n < 2) {
        serve 1
    }
    serve n * fact(n - 1)
}
fact(5)
`, 0)
	timing := rec.Timing()
	kpis := timing.KPIs()
	k, ok := kpiByName(kpis, "fact(...)")
	if !ok {
		t.Fatalf("no KPI bucket for fact(...); kpis = %+v", kpis)
	}
	// fact(5) calls fact(4)...fact(1): 5 recursive calls total, all one family.
	if k.Calls != 5 {
		t.Errorf("Calls = %d, want 5 (every recursion depth counted, all in one bucket)", k.Calls)
	}
}

func TestTimingKPIsGroupAllLapsOfOneLoopTogether(t *testing.T) {
	rec := record(t, `
total = 0
knead n in [1, 2, 3, 4, 5, 6, 7, 8] {
    total += n
}
`, 0)
	timing := rec.Timing()
	kpis := timing.KPIs()
	var loopKPI *KPI
	for i := range kpis {
		if kpis[i].Name != TopLevelFamily {
			loopKPI = &kpis[i]
		}
	}
	if loopKPI == nil {
		t.Fatalf("no loop KPI bucket found; kpis = %+v", kpis)
	}
	if loopKPI.Calls != 8 {
		t.Errorf("Calls = %d, want 8 laps, all in one bucket (even though the tree folds them for display)", loopKPI.Calls)
	}
}

func TestTimingSelfExcludesNestedFrameTime(t *testing.T) {
	// A recursive call's total duration nests every level beneath it;
	// Self for the outermost call site should not include the inner
	// calls' own work twice over. We can't assert exact durations
	// (too flaky), but Self must never exceed Total for any node, and
	// the sum of every root's Total must equal Overall() exactly (no
	// double counting at the top level).
	rec := record(t, `
recipe fact(n) {
    order (n < 2) {
        serve 1
    }
    serve n * fact(n - 1)
}
fact(6)
`, 0)
	timing := rec.Timing()
	var sum int64
	for _, root := range rec.Roots() {
		nt := timing.Of(root)
		if nt.Self > nt.Total {
			t.Errorf("root %q: Self (%v) > Total (%v)", root.Label(), nt.Self, nt.Total)
		}
		sum += int64(nt.Total)
	}
	if sum != int64(timing.Overall()) {
		t.Errorf("sum of root Totals = %d, want exactly Overall() = %d", sum, timing.Overall())
	}
}

func TestTimingKPIsRankedBySelfTimeDescending(t *testing.T) {
	rec := record(t, `
recipe slow(n) {
    total = 0
    knead i in 0.<n {
        total += i
    }
    serve total
}
recipe fast(n) {
    serve n
}
slow(50)
fast(1)
`, 0)
	timing := rec.Timing()
	kpis := timing.KPIs()
	for i := 1; i < len(kpis); i++ {
		if kpis[i].SelfTime > kpis[i-1].SelfTime {
			t.Errorf("kpis not ranked by SelfTime descending at index %d: %+v", i, kpis)
		}
	}
}

func TestTimingKPISelfSizeAccumulates(t *testing.T) {
	rec := record(t, `xs = [1, 2, 3, 4, 5]`, 0)
	timing := rec.Timing()
	kpis := timing.KPIs()
	k, ok := kpiByName(kpis, TopLevelFamily)
	if !ok {
		t.Fatal("no top-level KPI bucket")
	}
	if k.SelfSize != 5 {
		t.Errorf("SelfSize = %d, want 5 (the List's element count)", k.SelfSize)
	}
}

func TestTimingKPIFailedCounts(t *testing.T) {
	rec := record(t, `
recipe boom() {
    serve 1 / 0
}
boom()
`, 0)
	timing := rec.Timing()
	kpis := timing.KPIs()
	k, ok := kpiByName(kpis, "boom(...)")
	if !ok {
		t.Fatalf("no KPI bucket for boom(...); kpis = %+v", kpis)
	}
	if k.Failed != 1 {
		t.Errorf("Failed = %d, want 1", k.Failed)
	}
}
