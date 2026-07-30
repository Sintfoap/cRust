package object

import (
	"math"
	"strconv"
)

// Float is cRust's Float type (SPEC.md §2): a 64-bit value. Unlike
// Integer, there's no small-value cache here — floats are far less
// likely to recur as identical values the way loop counters do, and
// NaN's self-inequality makes a naive value-keyed cache actively
// wrong, so it's not worth the complexity of getting right.
type Float struct {
	Value float64
}

func (f *Float) Type() ObjectType { return FLOAT_OBJ }
func (f *Float) Inspect() string  { return strconv.FormatFloat(f.Value, 'g', -1, 64) }

func (f *Float) HashKey() HashKey {
	return HashKey{Type: FLOAT_OBJ, Value: math.Float64bits(f.Value)}
}
