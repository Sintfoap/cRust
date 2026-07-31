package token

import "testing"

func TestLookupIdent(t *testing.T) {
	tests := []struct {
		ident string
		want  Type
	}{
		{"recipe", RECIPE},
		{"order", ORDER},
		{"combo", COMBO},
		{"special", SPECIAL},
		{"knead", KNEAD},
		{"in", IN},
		{"bake", BAKE},
		{"burnt", BURNT},
		{"flip", FLIP},
		{"serve", SERVE},
		{"stuffed", STUFFED},
		{"thin", THIN},
		{"nobox", NOBOX},
		{"toppings", TOPPINGS},
		{"with", WITH},
		{"or", OR},
		{"hold", HOLD},
		// Not keywords: builtins are predeclared identifiers, not
		// reserved words (SPEC.md §4), and topping/sauce were dropped
		// as keywords entirely (SPEC.md §3/§4).
		{"deliver", IDENT},
		{"slices", IDENT},
		{"sauce", IDENT},
		{"topping", IDENT},
		{"gather", IDENT},
		{"myVar", IDENT},
		{"Recipe", IDENT}, // keywords are case-sensitive
	}

	for _, tt := range tests {
		t.Run(tt.ident, func(t *testing.T) {
			if got := LookupIdent(tt.ident); got != tt.want {
				t.Errorf("LookupIdent(%q) = %v, want %v", tt.ident, got, tt.want)
			}
		})
	}
}

func TestKeywords(t *testing.T) {
	kws := Keywords()
	if len(kws) == 0 {
		t.Fatal("Keywords() returned an empty map")
	}
	for spelling, want := range kws {
		if got := LookupIdent(spelling); got != want {
			t.Errorf("Keywords()[%q] = %v, but LookupIdent(%q) = %v", spelling, want, spelling, got)
		}
	}
	if kws["recipe"] != RECIPE {
		t.Errorf(`Keywords()["recipe"] = %v, want RECIPE`, kws["recipe"])
	}
}
