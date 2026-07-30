package parser

import (
	"strings"
	"testing"

	"github.com/Sintfoap/cRust/internal/lexer"
)

// mustHaveErrors parses input and fails the test if parsing produced no
// errors — used for the malformed-input side of the parser's contract,
// mirroring how parser_test.go's parseProgram/checkParserErrors cover
// the well-formed side.
func mustHaveErrors(t *testing.T, input string) {
	t.Helper()
	l := lexer.New(input)
	p := New(l)
	p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Errorf("input %q: expected parse error(s), got none", input)
	}
}

func TestMalformedInputProducesErrors(t *testing.T) {
	tests := []string{
		// order/combo/special
		"order x < y) { serve 1 }\n",
		"order (x < y { serve 1 }\n",
		"order (x < y)\n",
		"order (x < y) { serve 1 } combo x { serve 2 }\n",
		"order (x < y) { serve 1 } special\n",

		// knead counted
		"knead (i = 0; i < 10; i++ { serve i }\n",
		"knead (i = 0 i < 10; i++) { serve i }\n",
		"knead (i = 0; i < 10; i++)\n",

		// knead forEach
		"knead item xs { serve item }\n",
		"knead in xs { serve item }\n",
		"knead item in xs\n",

		// bake
		"bake x < 10 { x++ }\n",
		"bake (x < 10\n",
		"bake (x < 10)\n",

		// blocks
		"{\nx = 1\n",

		// recipe / function literals
		"recipe f(x y) { serve x }\n",
		"recipe f(x, { serve x }\n",
		"recipe f(x)\n",
		"apply = recipe(x { serve x }\n",

		// calls, indexing, grouping
		"add(1, 2\n",
		"arr[0\n",
		"(1 + 2\n",
		"[1, 2\n",

		// map / set literals
		"m = {\"a\" 1}\n",
		"m = {\"a\": 1\n",
		"s = toppings[1, 2]\n",
		"s = toppings{1, 2\n",

		// ternary / range
		"x = cond (| a b\n",
		"x = 1..5..10\n",

		// unpack assign
		"a, = xs\n",
		"a, b xs\n",

		// invalid lvalues
		"1 + 2 = 3\n",
		"(1 + 2)++\n",
		"++5\n",

		// terminator errors
		"x = 1 y = 2\n",

		// no prefix parse function
		":\n",
		")\n",

		// integer literal overflow
		"99999999999999999999999999999999\n",

		// float literal overflow (ErrRange from strconv.ParseFloat)
		strings.Repeat("9", 400) + ".1\n",
	}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			mustHaveErrors(t, input)
		})
	}
}

func TestUnterminatedBlockRecordsError(t *testing.T) {
	mustHaveErrors(t, "recipe f() { serve 1\n")
}
