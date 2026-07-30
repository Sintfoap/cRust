package object

import "strconv"

// Integer is cRust's Integer type (SPEC.md §2): a 64-bit signed value.
type Integer struct {
	Value int64
}

func (i *Integer) Type() ObjectType { return INTEGER_OBJ }
func (i *Integer) Inspect() string  { return strconv.FormatInt(i.Value, 10) }

func (i *Integer) HashKey() HashKey {
	return HashKey{Type: INTEGER_OBJ, Value: uint64(i.Value)}
}

// intCacheMin/intCacheMax bound the pre-allocated small-integer cache.
// The range leans toward what AoC code actually produces: loop
// counters and indices (mostly small non-negative numbers) and
// neighbor-offset deltas (-1, 0, 1 turn up constantly in grid-walking
// puzzles). Values outside the range still work correctly — NewInteger
// just falls back to a normal allocation for them.
const (
	intCacheMin = -256
	intCacheMax = 256
)

var intCache [intCacheMax - intCacheMin + 1]Integer

func init() {
	for i := range intCache {
		intCache[i].Value = int64(i + intCacheMin)
	}
}

// NewInteger returns an *Integer for value, reusing a cached instance
// when value falls in the small-integer range instead of allocating.
// This is safe because cRust Integers are immutable — nothing can
// observe two equal values sharing a pointer, since there's no way to
// mutate an Integer's Value in place (assignment rebinds a variable to
// a — possibly different, possibly cached — Integer, it never edits an
// existing one).
func NewInteger(value int64) *Integer {
	if value >= intCacheMin && value <= intCacheMax {
		return &intCache[value-intCacheMin]
	}
	return &Integer{Value: value}
}
