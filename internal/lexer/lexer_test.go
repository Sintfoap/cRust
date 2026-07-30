package lexer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Sintfoap/cRust/internal/token"
)

type expected struct {
	typ     token.Type
	literal string
}

func assertTokens(t *testing.T, input string, want []expected) {
	t.Helper()
	l := New(input)
	for i, w := range want {
		got := l.NextToken()
		if got.Type != w.typ || got.Literal != w.literal {
			t.Fatalf("token[%d] = {%v %q}, want {%v %q}", i, got.Type, got.Literal, w.typ, w.literal)
		}
	}
}

func TestOperatorsAndDelimiters(t *testing.T) {
	input := `+ - * / % ++ -- += -= *= /= %= = == != < > <= >= ( ) { } [ ] , : ; (| |) ?: .. .<`

	want := []expected{
		{token.PLUS, "+"},
		{token.MINUS, "-"},
		{token.STAR, "*"},
		{token.SLASH, "/"},
		{token.PERCENT, "%"},
		{token.INC, "++"},
		{token.DEC, "--"},
		{token.PLUS_ASSIGN, "+="},
		{token.MINUS_ASSIGN, "-="},
		{token.STAR_ASSIGN, "*="},
		{token.SLASH_ASSIGN, "/="},
		{token.PERCENT_ASSIGN, "%="},
		{token.ASSIGN, "="},
		{token.EQ, "=="},
		{token.NOT_EQ, "!="},
		{token.LT, "<"},
		{token.GT, ">"},
		{token.LE, "<="},
		{token.GE, ">="},
		{token.LPAREN, "("},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.RBRACE, "}"},
		{token.LBRACKET, "["},
		{token.RBRACKET, "]"},
		{token.COMMA, ","},
		{token.COLON, ":"},
		{token.SEMICOLON, ";"},
		{token.TERN_THEN, "(|"},
		{token.TERN_ELSE, "|)"},
		{token.ELVIS, "?:"},
		{token.DOTDOT, ".."},
		{token.DOTLT, ".<"},
		{token.EOF, ""},
	}

	assertTokens(t, input, want)
}

func TestKeywordsAndIdentifiers(t *testing.T) {
	input := `recipe order combo special knead in bake burnt flip serve stuffed thin nobox toppings with or hold
deliver slices sauce myVar _underscore camelCase42`

	want := []expected{
		{token.RECIPE, "recipe"},
		{token.ORDER, "order"},
		{token.COMBO, "combo"},
		{token.SPECIAL, "special"},
		{token.KNEAD, "knead"},
		{token.IN, "in"},
		{token.BAKE, "bake"},
		{token.BURNT, "burnt"},
		{token.FLIP, "flip"},
		{token.SERVE, "serve"},
		{token.STUFFED, "stuffed"},
		{token.THIN, "thin"},
		{token.NOBOX, "nobox"},
		{token.TOPPINGS, "toppings"},
		{token.WITH, "with"},
		{token.OR, "or"},
		{token.HOLD, "hold"},
		{token.NEWLINE, "\n"},
		{token.IDENT, "deliver"},
		{token.IDENT, "slices"},
		{token.IDENT, "sauce"},
		{token.IDENT, "myVar"},
		{token.IDENT, "_underscore"},
		{token.IDENT, "camelCase42"},
		{token.EOF, ""},
	}

	assertTokens(t, input, want)
}

func TestNumbers(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []expected
	}{
		{"int", "42", []expected{{token.INT, "42"}, {token.EOF, ""}}},
		{"float", "3.14", []expected{{token.FLOAT, "3.14"}, {token.EOF, ""}}},
		{"zero", "0", []expected{{token.INT, "0"}, {token.EOF, ""}}},
		{
			"inclusive range",
			"1..5",
			[]expected{{token.INT, "1"}, {token.DOTDOT, ".."}, {token.INT, "5"}, {token.EOF, ""}},
		},
		{
			"exclusive range",
			"1.<5",
			[]expected{{token.INT, "1"}, {token.DOTLT, ".<"}, {token.INT, "5"}, {token.EOF, ""}},
		},
		{
			"float then range is a parse-time concern, not lexer ambiguity",
			"3.14..5",
			[]expected{{token.FLOAT, "3.14"}, {token.DOTDOT, ".."}, {token.INT, "5"}, {token.EOF, ""}},
		},
		{
			"range on a paren'd sum",
			"(1+1)..(5+1)",
			[]expected{
				{token.LPAREN, "("}, {token.INT, "1"}, {token.PLUS, "+"}, {token.INT, "1"}, {token.RPAREN, ")"},
				{token.DOTDOT, ".."},
				{token.LPAREN, "("}, {token.INT, "5"}, {token.PLUS, "+"}, {token.INT, "1"}, {token.RPAREN, ")"},
				{token.EOF, ""},
			},
		},
		{
			"unary minus is not part of the number",
			"-7",
			[]expected{{token.MINUS, "-"}, {token.INT, "7"}, {token.EOF, ""}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertTokens(t, tt.input, tt.want)
		})
	}
}

func TestStrings(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []expected
	}{
		{
			"plain",
			`"pepperoni"`,
			[]expected{{token.STRING, "pepperoni"}, {token.EOF, ""}},
		},
		{
			"empty",
			`""`,
			[]expected{{token.STRING, ""}, {token.EOF, ""}},
		},
		{
			"escapes",
			`"line1\nline2\ttab\\slash\"quote"`,
			[]expected{{token.STRING, "line1\nline2\ttab\\slash\"quote"}, {token.EOF, ""}},
		},
		{
			"unterminated at EOF",
			`"no closing quote`,
			[]expected{{token.ILLEGAL, "unterminated string literal"}},
		},
		{
			"unterminated at newline",
			"\"no closing quote\nrest",
			[]expected{{token.ILLEGAL, "unterminated string literal"}},
		},
		{
			"trailing backslash before EOF",
			`"oops\`,
			[]expected{{token.ILLEGAL, "unterminated string literal"}},
		},
		{
			"unknown escape",
			`"bad \q escape"`,
			[]expected{{token.ILLEGAL, `unknown escape sequence \q`}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertTokens(t, tt.input, tt.want)
		})
	}
}

func TestComments(t *testing.T) {
	input := `deliver("hi") // trailing comment
// a whole-line comment
serve nobox`

	want := []expected{
		{token.IDENT, "deliver"},
		{token.LPAREN, "("},
		{token.STRING, "hi"},
		{token.RPAREN, ")"},
		{token.NEWLINE, "\n"},
		{token.SERVE, "serve"},
		{token.NOBOX, "nobox"},
		{token.EOF, ""},
	}

	assertTokens(t, input, want)
}

func TestNewlineCollapsing(t *testing.T) {
	input := "a = 1\n\n\n// blank lines and a comment above\n\nb = 2"

	want := []expected{
		{token.IDENT, "a"},
		{token.ASSIGN, "="},
		{token.INT, "1"},
		{token.NEWLINE, "\n"},
		{token.IDENT, "b"},
		{token.ASSIGN, "="},
		{token.INT, "2"},
		{token.EOF, ""},
	}

	assertTokens(t, input, want)
}

func TestLineContinuationInsideBrackets(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []expected
	}{
		{
			name:  "multi-line call arguments",
			input: "add(\n1,\n2\n)",
			want: []expected{
				{token.IDENT, "add"}, {token.LPAREN, "("},
				{token.INT, "1"}, {token.COMMA, ","},
				{token.INT, "2"},
				{token.RPAREN, ")"},
				{token.EOF, ""},
			},
		},
		{
			name:  "multi-line list literal",
			input: "[\n1,\n2\n]",
			want: []expected{
				{token.LBRACKET, "["},
				{token.INT, "1"}, {token.COMMA, ","},
				{token.INT, "2"},
				{token.RBRACKET, "]"},
				{token.EOF, ""},
			},
		},
		{
			name:  "nested brackets combine depth",
			input: "f([\n1,\n2\n])",
			want: []expected{
				{token.IDENT, "f"}, {token.LPAREN, "("}, {token.LBRACKET, "["},
				{token.INT, "1"}, {token.COMMA, ","},
				{token.INT, "2"},
				{token.RBRACKET, "]"}, {token.RPAREN, ")"},
				{token.EOF, ""},
			},
		},
		{
			name:  "newline significant again once brackets close",
			input: "f(\n1\n)\nx = 2",
			want: []expected{
				{token.IDENT, "f"}, {token.LPAREN, "("},
				{token.INT, "1"},
				{token.RPAREN, ")"},
				{token.NEWLINE, "\n"},
				{token.IDENT, "x"}, {token.ASSIGN, "="}, {token.INT, "2"},
				{token.EOF, ""},
			},
		},
		{
			name:  "a comment inside an unclosed paren is still skipped",
			input: "f(\n// comment\n1\n)",
			want: []expected{
				{token.IDENT, "f"}, {token.LPAREN, "("},
				{token.INT, "1"},
				{token.RPAREN, ")"},
				{token.EOF, ""},
			},
		},
		{
			name:  "unbalanced closing paren doesn't underflow depth",
			input: ")\nx = 1",
			want: []expected{
				{token.RPAREN, ")"},
				{token.NEWLINE, "\n"},
				{token.IDENT, "x"}, {token.ASSIGN, "="}, {token.INT, "1"},
				{token.EOF, ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertTokens(t, tt.input, tt.want)
		})
	}
}

// TestBlockNewlinesStayInsideBraces guards against the '(' / '['
// line-continuation rule accidentally leaking into '{' — brace-delimited
// blocks must keep treating every '\n' as a real statement terminator,
// even while nested inside an open '(' (a recipe declared inside a
// call's argument list, however contrived, shouldn't lose statement
// separation in its body).
func TestBlockNewlinesStayInsideBraces(t *testing.T) {
	input := "f(recipe() {\nx = 1\ny = 2\n})"

	want := []expected{
		{token.IDENT, "f"}, {token.LPAREN, "("},
		{token.RECIPE, "recipe"}, {token.LPAREN, "("}, {token.RPAREN, ")"}, {token.LBRACE, "{"},
		{token.NEWLINE, "\n"},
		{token.IDENT, "x"}, {token.ASSIGN, "="}, {token.INT, "1"}, {token.NEWLINE, "\n"},
		{token.IDENT, "y"}, {token.ASSIGN, "="}, {token.INT, "2"}, {token.NEWLINE, "\n"},
		{token.RBRACE, "}"}, {token.RPAREN, ")"},
		{token.EOF, ""},
	}

	assertTokens(t, input, want)
}

func TestIllegalCharacters(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  expected
	}{
		{"bang alone", "!", expected{token.ILLEGAL, `unexpected character '!' (did you mean 'hold' for logical not?)`}},
		{"lone pipe", "|", expected{token.ILLEGAL, `unexpected character '|'`}},
		{"lone question mark", "?", expected{token.ILLEGAL, `unexpected character '?'`}},
		{"lone dot", ".", expected{token.ILLEGAL, `unexpected character '.'`}},
		{"at sign", "@", expected{token.ILLEGAL, `unexpected character '@'`}},
		{"ampersand", "&", expected{token.ILLEGAL, `unexpected character '&'`}},
		{"dollar", "$", expected{token.ILLEGAL, `unexpected character '$'`}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertTokens(t, tt.input, []expected{tt.want})
		})
	}
}

func TestLineAndColumnTracking(t *testing.T) {
	input := "a = 1\nbb = 22\n"
	l := New(input)

	type pos struct {
		line, col int
		typ       token.Type
	}
	want := []pos{
		{1, 1, token.IDENT},  // a
		{1, 3, token.ASSIGN}, // =
		{1, 5, token.INT},    // 1
		{1, 6, token.NEWLINE},
		{2, 1, token.IDENT},  // bb
		{2, 4, token.ASSIGN}, // =
		{2, 6, token.INT},    // 22
		{2, 8, token.NEWLINE},
		{3, 1, token.EOF},
	}

	for i, w := range want {
		got := l.NextToken()
		if got.Type != w.typ || got.Line != w.line || got.Col != w.col {
			t.Fatalf("token[%d] = {%v line=%d col=%d}, want {%v line=%d col=%d}",
				i, got.Type, got.Line, got.Col, w.typ, w.line, w.col)
		}
	}
}

// TestFullProgram tokenizes the Hello World example end-to-end, as a
// sanity check that nothing about real .crust source trips the lexer
// up in combination, not just in isolated cases.
func TestFullProgram(t *testing.T) {
	input := `recipe findPair(nums, target) {
    knead (i = 0; i < slices(nums); i++) {
        order (nums[i] == target) {
            serve i
        }
    }
    serve nobox
}`

	want := []expected{
		{token.RECIPE, "recipe"},
		{token.IDENT, "findPair"},
		{token.LPAREN, "("},
		{token.IDENT, "nums"},
		{token.COMMA, ","},
		{token.IDENT, "target"},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.NEWLINE, "\n"},

		{token.KNEAD, "knead"},
		{token.LPAREN, "("},
		{token.IDENT, "i"},
		{token.ASSIGN, "="},
		{token.INT, "0"},
		{token.SEMICOLON, ";"},
		{token.IDENT, "i"},
		{token.LT, "<"},
		{token.IDENT, "slices"},
		{token.LPAREN, "("},
		{token.IDENT, "nums"},
		{token.RPAREN, ")"},
		{token.SEMICOLON, ";"},
		{token.IDENT, "i"},
		{token.INC, "++"},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.NEWLINE, "\n"},

		{token.ORDER, "order"},
		{token.LPAREN, "("},
		{token.IDENT, "nums"},
		{token.LBRACKET, "["},
		{token.IDENT, "i"},
		{token.RBRACKET, "]"},
		{token.EQ, "=="},
		{token.IDENT, "target"},
		{token.RPAREN, ")"},
		{token.LBRACE, "{"},
		{token.NEWLINE, "\n"},

		{token.SERVE, "serve"},
		{token.IDENT, "i"},
		{token.NEWLINE, "\n"},

		{token.RBRACE, "}"},
		{token.NEWLINE, "\n"},

		{token.RBRACE, "}"},
		{token.NEWLINE, "\n"},

		{token.SERVE, "serve"},
		{token.NOBOX, "nobox"},
		{token.NEWLINE, "\n"},

		{token.RBRACE, "}"},
		{token.EOF, ""},
	}

	assertTokens(t, input, want)
}

// TestEOFIsSticky ensures repeated calls past the end of input don't
// panic or wrap around, since the parser will call NextToken() in a
// loop that only stops on seeing EOF.
func TestEOFIsSticky(t *testing.T) {
	l := New("x")
	l.NextToken() // IDENT x
	for i := 0; i < 3; i++ {
		tok := l.NextToken()
		if tok.Type != token.EOF {
			t.Fatalf("call %d: got %v, want EOF", i, tok.Type)
		}
	}
}

// TestExampleFiles tokenizes every hand-written .crust program under
// examples/ end-to-end. Those files exercise every language feature
// together (unpacking, ranges, ternary, Elvis, sets, for-each, ...),
// so this catches real-world gaps that isolated unit tests above might
// not — the bar is just "no ILLEGAL token and a clean EOF," since
// there's no parser yet to check anything deeper.
func TestExampleFiles(t *testing.T) {
	matches, err := filepath.Glob("../../examples/*.crust")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no .crust files found under examples/ — glob path likely wrong")
	}

	for _, path := range matches {
		t.Run(filepath.Base(path), func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			l := New(string(src))
			for {
				tok := l.NextToken()
				if tok.Type == token.ILLEGAL {
					t.Fatalf("ILLEGAL token %q at line %d col %d", tok.Literal, tok.Line, tok.Col)
				}
				if tok.Type == token.EOF {
					break
				}
			}
		})
	}
}
