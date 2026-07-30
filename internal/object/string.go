package object

import "hash/fnv"

// String is cRust's String type (SPEC.md §2).
type String struct {
	Value string
}

func (s *String) Type() ObjectType { return STRING_OBJ }
func (s *String) Inspect() string  { return s.Value }

func (s *String) HashKey() HashKey {
	h := fnv.New64a()
	h.Write([]byte(s.Value))
	return HashKey{Type: STRING_OBJ, Value: h.Sum64()}
}
