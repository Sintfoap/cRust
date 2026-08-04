package object

import "testing"

func TestGridGetInBounds(t *testing.T) {
	g := NewGrid([][]Object{
		{NewInteger(1), NewInteger(2)},
		{NewInteger(3), NewInteger(4)},
	}, 0, 0)
	v, ok := g.Get(1, 0)
	if !ok {
		t.Fatal("expected (1,0) to be in bounds")
	}
	if v.(*Integer).Value != 3 {
		t.Errorf("Get(1,0) = %v, want 3", v)
	}
}

func TestGridGetOutOfBoundsIsFalse(t *testing.T) {
	g := NewGrid([][]Object{{NewInteger(1)}}, 0, 0)
	if _, ok := g.Get(5, 5); ok {
		t.Error("Get(5,5) = true, want false (out of bounds)")
	}
	if _, ok := g.Get(-1, 0); ok {
		t.Error("Get(-1,0) = true, want false (row negative, out of bounds)")
	}
	if _, ok := g.Get(0, 5); ok {
		t.Error("Get(0,5) = true, want false (row in bounds, col out of bounds)")
	}
}

func TestGridGetRespectsOffset(t *testing.T) {
	// Rows[0][0] is logical (-3, -3), not (0, 0).
	g := NewGrid([][]Object{{NewInteger(42)}}, -3, -3)
	v, ok := g.Get(-3, -3)
	if !ok || v.(*Integer).Value != 42 {
		t.Fatalf("Get(-3,-3) = %v, %v, want 42, true", v, ok)
	}
	if _, ok := g.Get(0, 0); ok {
		t.Error("Get(0,0) should be out of bounds on a grid whose only cell is (-3,-3)")
	}
}

func TestGridSetWithinBoundsMutatesInPlace(t *testing.T) {
	g := NewGrid([][]Object{
		{NewInteger(1), NewInteger(2)},
		{NewInteger(3), NewInteger(4)},
	}, 0, 0)
	g.Set(0, 1, NewInteger(99))
	v, _ := g.Get(0, 1)
	if v.(*Integer).Value != 99 {
		t.Errorf("Get(0,1) after Set = %v, want 99", v)
	}
	if g.Height() != 2 || g.Width() != 2 {
		t.Errorf("Height/Width = %d/%d, want 2/2 (no expansion should have happened)", g.Height(), g.Width())
	}
}

func TestGridSetOnEmptyGridStartsAtThatCoordinate(t *testing.T) {
	g := &Grid{}
	g.Set(5, -5, NewInteger(1))
	if g.Height() != 1 || g.Width() != 1 {
		t.Fatalf("Height/Width = %d/%d, want 1/1", g.Height(), g.Width())
	}
	v, ok := g.Get(5, -5)
	if !ok || v.(*Integer).Value != 1 {
		t.Errorf("Get(5,-5) = %v, %v, want 1, true", v, ok)
	}
}

func TestGridSetExpandsPositively(t *testing.T) {
	g := NewGrid([][]Object{{NewInteger(1)}}, 0, 0)
	g.Set(2, 3, NewInteger(9))
	if g.Height() != 3 || g.Width() != 4 {
		t.Fatalf("Height/Width = %d/%d, want 3/4", g.Height(), g.Width())
	}
	if v, ok := g.Get(0, 0); !ok || v.(*Integer).Value != 1 {
		t.Errorf("original cell (0,0) lost after expansion: %v, %v", v, ok)
	}
	if v, ok := g.Get(2, 3); !ok || v.(*Integer).Value != 9 {
		t.Errorf("Get(2,3) = %v, %v, want 9, true", v, ok)
	}
	// Newly created cells default to nobox.
	if v, ok := g.Get(1, 1); !ok || v != NULL {
		t.Errorf("Get(1,1) (never written) = %v, %v, want NULL, true", v, ok)
	}
}

func TestGridSetExpandsNegativelyAndShiftsOffset(t *testing.T) {
	g := NewGrid([][]Object{{NewInteger(1)}}, 0, 0)
	g.Set(-2, -2, NewInteger(7))
	if g.RowOffset != -2 || g.ColOffset != -2 {
		t.Fatalf("RowOffset/ColOffset = %d/%d, want -2/-2", g.RowOffset, g.ColOffset)
	}
	if g.Height() != 3 || g.Width() != 3 {
		t.Fatalf("Height/Width = %d/%d, want 3/3", g.Height(), g.Width())
	}
	// The original (0,0) must still mean the same thing after the shift.
	if v, ok := g.Get(0, 0); !ok || v.(*Integer).Value != 1 {
		t.Errorf("Get(0,0) after negative expansion = %v, %v, want 1, true (offset shift must preserve prior coordinates)", v, ok)
	}
	if v, ok := g.Get(-2, -2); !ok || v.(*Integer).Value != 7 {
		t.Errorf("Get(-2,-2) = %v, %v, want 7, true", v, ok)
	}
}

func TestGridSetExpandsInMixedDirections(t *testing.T) {
	g := NewGrid([][]Object{{NewInteger(1)}}, 0, 0)
	g.Set(-1, 3, NewInteger(2)) // negative row, positive col, same call
	if v, ok := g.Get(0, 0); !ok || v.(*Integer).Value != 1 {
		t.Errorf("original cell lost: %v, %v", v, ok)
	}
	if v, ok := g.Get(-1, 3); !ok || v.(*Integer).Value != 2 {
		t.Errorf("Get(-1,3) = %v, %v, want 2, true", v, ok)
	}
}

func TestGridSetSequentialExpansionInBothDirections(t *testing.T) {
	// Simulates the motivating use case: repeated setAt calls walking
	// outward from an origin in every direction, as a Game-of-Life-style
	// simulation would.
	g := &Grid{}
	g.Set(0, 0, NewInteger(0))
	g.Set(-1, -1, NewInteger(1))
	g.Set(1, 1, NewInteger(2))
	g.Set(-1, 1, NewInteger(3))
	g.Set(1, -1, NewInteger(4))

	tests := []struct {
		row, col int
		want     int64
	}{
		{0, 0, 0}, {-1, -1, 1}, {1, 1, 2}, {-1, 1, 3}, {1, -1, 4},
	}
	for _, tt := range tests {
		v, ok := g.Get(tt.row, tt.col)
		if !ok || v.(*Integer).Value != tt.want {
			t.Errorf("Get(%d,%d) = %v, %v, want %d, true", tt.row, tt.col, v, ok, tt.want)
		}
	}
	if g.Height() != 3 || g.Width() != 3 {
		t.Errorf("Height/Width = %d/%d, want 3/3", g.Height(), g.Width())
	}
}

func TestGridBoundsEmpty(t *testing.T) {
	g := &Grid{}
	_, _, _, _, ok := g.Bounds()
	if ok {
		t.Error("Bounds() on an empty grid should report ok=false")
	}
}

func TestGridBoundsAfterExpansion(t *testing.T) {
	g := NewGrid([][]Object{{NewInteger(1)}}, 0, 0)
	g.Set(-2, 3, NewInteger(1))
	minRow, minCol, maxRow, maxCol, ok := g.Bounds()
	if !ok {
		t.Fatal("expected ok=true")
	}
	if minRow != -2 || minCol != 0 || maxRow != 0 || maxCol != 3 {
		t.Errorf("Bounds() = (%d,%d,%d,%d), want (-2,0,0,3)", minRow, minCol, maxRow, maxCol)
	}
}

func TestGridInspect(t *testing.T) {
	g := NewGrid([][]Object{
		{NewInteger(1), NewInteger(2)},
		{&String{Value: "x"}, NULL},
	}, -5, -5) // offset must not show up in Inspect()
	want := `[[1, 2], [x, nobox]]`
	if got := g.Inspect(); got != want {
		t.Errorf("Inspect() = %q, want %q", got, want)
	}
}

func TestGridType(t *testing.T) {
	g := &Grid{}
	if g.Type() != GRID_OBJ {
		t.Errorf("Type() = %v, want GRID_OBJ", g.Type())
	}
}

func TestGridHeightWidthOnEmptyGrid(t *testing.T) {
	g := &Grid{}
	if g.Height() != 0 || g.Width() != 0 {
		t.Errorf("Height/Width on empty grid = %d/%d, want 0/0", g.Height(), g.Width())
	}
}
