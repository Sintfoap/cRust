package object

import "strings"

// Grid is cRust's Grid type (SPEC.md §2.4): a mutable 2D structure
// whose logical (row, col) coordinates can be negative and grow in
// any direction — what `setAt` needs to auto-expand a grid instead of
// erroring on an out-of-range write, which a plain List-of-List can't
// do: a List has no room to remember "row 0 currently means logical
// row -3" between calls, and expanding into negative coordinates
// means exactly that has to persist across every setAt call on the
// same grid. RowOffset/ColOffset are that persistent memory: the
// logical coordinate of Rows[0][0].
//
// Deliberately its own type rather than reusing List-of-List (which
// `grid(s)` returned before this existed) — see
// ARCHITECTURE.md's Grid section for why a plain List couldn't do
// this and what had to change as a result (grid()'s return type,
// at/setAt no longer being "just List indexing with a friendlier
// error").
type Grid struct {
	Rows      [][]Object
	RowOffset int
	ColOffset int
}

// NewGrid returns a Grid over rows (already offset by rowOffset,
// colOffset) — every row must be the same length; callers that build
// rows by hand (grid(s), newGrid()) are the only ones responsible for
// upholding that, same as object.Map's key-type invariant is upheld by
// its own callers rather than checked here on every access.
func NewGrid(rows [][]Object, rowOffset, colOffset int) *Grid {
	return &Grid{Rows: rows, RowOffset: rowOffset, ColOffset: colOffset}
}

func (g *Grid) Type() ObjectType { return GRID_OBJ }

// Inspect renders a Grid exactly the way the List-of-List it replaced
// used to (`[[a, b], [c, d]]`) — RowOffset/ColOffset are bookkeeping
// for where a logical coordinate lands internally, not something
// SPEC.md gives the user any syntax to see or set directly, so they
// stay out of the rendered form the same way a Map's internal bucket
// layout would.
func (g *Grid) Inspect() string {
	var out strings.Builder
	out.WriteByte('[')
	for i, row := range g.Rows {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteByte('[')
		for j, cell := range row {
			if j > 0 {
				out.WriteString(", ")
			}
			out.WriteString(cell.Inspect())
		}
		out.WriteByte(']')
	}
	out.WriteByte(']')
	return out.String()
}

// Height and Width report the grid's current row/column counts (0 for
// an empty grid, which has no rows at all yet).
func (g *Grid) Height() int { return len(g.Rows) }
func (g *Grid) Width() int {
	if len(g.Rows) == 0 {
		return 0
	}
	return len(g.Rows[0])
}

// Get reads the value at logical (row, col), and whether that
// coordinate is currently in bounds — false, not an error, so callers
// (atFn) can turn a miss into nobox the same way a missing Map key
// already does.
func (g *Grid) Get(row, col int) (Object, bool) {
	r := row - g.RowOffset
	c := col - g.ColOffset
	if r < 0 || r >= len(g.Rows) {
		return nil, false
	}
	if c < 0 || c >= len(g.Rows[r]) {
		return nil, false
	}
	return g.Rows[r][c], true
}

// Set writes value at logical (row, col), growing the grid in
// whichever direction(s) — including negative — that coordinate falls
// outside the current bounds.
//
// The whole grid is rebuilt into a new bounding box (rather than
// incrementally prepending/appending rows and columns) whenever the
// target falls outside the current one: simpler and less error-prone
// than four separate incremental-growth cases (row before, row after,
// col before, col after, and any combination of them at once), at the
// cost of an O(new area) copy on every out-of-bounds write rather than
// O(one new row/column). Fine for cRust's AoC-sized inputs (the same
// tradeoff SPEC.md's Performance Strategy already makes elsewhere) —
// a grid growing one cell at a time from repeated setAt calls near its
// edge is the common case this exists for, not a single call jumping
// the bounding box by a huge amount.
func (g *Grid) Set(row, col int, value Object) {
	if len(g.Rows) == 0 {
		g.Rows = [][]Object{{value}}
		g.RowOffset, g.ColOffset = row, col
		return
	}

	height, width := g.Height(), g.Width()
	rowMin := min(g.RowOffset, row)
	rowMax := max(g.RowOffset+height-1, row)
	colMin := min(g.ColOffset, col)
	colMax := max(g.ColOffset+width-1, col)

	if rowMin != g.RowOffset || colMin != g.ColOffset || rowMax-rowMin+1 != height || colMax-colMin+1 != width {
		newHeight, newWidth := rowMax-rowMin+1, colMax-colMin+1
		newRows := make([][]Object, newHeight)
		for i := range newRows {
			newRows[i] = make([]Object, newWidth)
			for j := range newRows[i] {
				newRows[i][j] = NULL
			}
		}
		rowShift := g.RowOffset - rowMin
		colShift := g.ColOffset - colMin
		for i, oldRow := range g.Rows {
			copy(newRows[i+rowShift][colShift:colShift+width], oldRow)
		}
		g.Rows = newRows
		g.RowOffset = rowMin
		g.ColOffset = colMin
	}

	g.Rows[row-g.RowOffset][col-g.ColOffset] = value
}

// Bounds returns the grid's current logical bounding box — (minRow,
// minCol, maxRow, maxCol), inclusive — and false for an empty grid,
// which has no bounds to report. This is how a reader learns where
// (0, 0) sits after any number of expanding setAt calls, since Grid
// deliberately isn't directly indexable/iterable the way a List is
// (see ARCHITECTURE.md): at/setAt/Bounds are the whole interface.
func (g *Grid) Bounds() (minRow, minCol, maxRow, maxCol int, ok bool) {
	if len(g.Rows) == 0 {
		return 0, 0, 0, 0, false
	}
	return g.RowOffset, g.ColOffset, g.RowOffset + g.Height() - 1, g.ColOffset + g.Width() - 1, true
}
