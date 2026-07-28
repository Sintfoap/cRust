// Package lexer turns cRust source text into a stream of tokens. See
// ARCHITECTURE.md's Phase 2 section for the design this implements,
// most importantly the number-vs-range ('.') and paren/pipe-vs-ternary
// ('(|'/'|)') disambiguation rules.
package lexer

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/Sintfoap/cRust/internal/token"
)

// Lexer is a single-pass, rune-by-rune scanner. Callers pull tokens
// one at a time via NextToken(); there's no pre-materialized slice.
type Lexer struct {
	input   []rune
	pos     int  // position of ch in input
	readPos int  // position of the next rune to read
	ch      rune // current rune under examination; 0 means EOF
	line    int  // 1-indexed line of ch
	col     int  // 1-indexed column of ch
}

// New returns a Lexer positioned at the first rune of input.
func New(input string) *Lexer {
	l := &Lexer{input: []rune(input), line: 1, col: 0}
	l.readChar()
	return l
}

// readChar advances the lexer by one rune, updating line/col. It's
// called with l.ch equal to the rune being moved *away* from, so a
// newline there means the rune we're moving *to* starts a new line.
func (l *Lexer) readChar() {
	if l.ch == '\n' {
		l.line++
		l.col = 0
	}
	if l.readPos >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPos]
	}
	l.pos = l.readPos
	l.readPos++
	l.col++
}

// peekChar returns the rune after l.ch without consuming it.
func (l *Lexer) peekChar() rune {
	if l.readPos >= len(l.input) {
		return 0
	}
	return l.input[l.readPos]
}

// NextToken scans and returns the next token, advancing past it.
// Calling NextToken repeatedly past the end of input keeps returning
// an EOF token.
func (l *Lexer) NextToken() token.Token {
	l.skipSpacesAndTabs()
	if l.ch == '/' && l.peekChar() == '/' {
		l.skipLineComment()
	}

	line, col := l.line, l.col

	switch l.ch {
	case 0:
		return token.Token{Type: token.EOF, Literal: "", Line: line, Col: col}

	case '\n':
		return l.readNewline(line, col)

	case '"':
		return l.readString(line, col)

	case '+':
		if l.peekChar() == '+' {
			l.readChar()
			return l.tok2(token.INC, "++", line, col)
		}
		if l.peekChar() == '=' {
			l.readChar()
			return l.tok2(token.PLUS_ASSIGN, "+=", line, col)
		}
		return l.tok1(token.PLUS, line, col)

	case '-':
		if l.peekChar() == '-' {
			l.readChar()
			return l.tok2(token.DEC, "--", line, col)
		}
		if l.peekChar() == '=' {
			l.readChar()
			return l.tok2(token.MINUS_ASSIGN, "-=", line, col)
		}
		return l.tok1(token.MINUS, line, col)

	case '*':
		if l.peekChar() == '=' {
			l.readChar()
			return l.tok2(token.STAR_ASSIGN, "*=", line, col)
		}
		return l.tok1(token.STAR, line, col)

	case '/':
		if l.peekChar() == '=' {
			l.readChar()
			return l.tok2(token.SLASH_ASSIGN, "/=", line, col)
		}
		return l.tok1(token.SLASH, line, col)

	case '%':
		if l.peekChar() == '=' {
			l.readChar()
			return l.tok2(token.PERCENT_ASSIGN, "%=", line, col)
		}
		return l.tok1(token.PERCENT, line, col)

	case '=':
		if l.peekChar() == '=' {
			l.readChar()
			return l.tok2(token.EQ, "==", line, col)
		}
		return l.tok1(token.ASSIGN, line, col)

	case '!':
		if l.peekChar() == '=' {
			l.readChar()
			return l.tok2(token.NOT_EQ, "!=", line, col)
		}
		return l.illegal(line, col, "unexpected character %q (did you mean 'hold' for logical not?)", l.ch)

	case '<':
		if l.peekChar() == '=' {
			l.readChar()
			return l.tok2(token.LE, "<=", line, col)
		}
		return l.tok1(token.LT, line, col)

	case '>':
		if l.peekChar() == '=' {
			l.readChar()
			return l.tok2(token.GE, ">=", line, col)
		}
		return l.tok1(token.GT, line, col)

	case '.':
		if l.peekChar() == '.' {
			l.readChar()
			return l.tok2(token.DOTDOT, "..", line, col)
		}
		if l.peekChar() == '<' {
			l.readChar()
			return l.tok2(token.DOTLT, ".<", line, col)
		}
		return l.illegal(line, col, "unexpected character %q", l.ch)

	case '?':
		if l.peekChar() == ':' {
			l.readChar()
			return l.tok2(token.ELVIS, "?:", line, col)
		}
		return l.illegal(line, col, "unexpected character %q", l.ch)

	case '(':
		if l.peekChar() == '|' {
			l.readChar()
			return l.tok2(token.TERN_THEN, "(|", line, col)
		}
		return l.tok1(token.LPAREN, line, col)

	case '|':
		if l.peekChar() == ')' {
			l.readChar()
			return l.tok2(token.TERN_ELSE, "|)", line, col)
		}
		return l.illegal(line, col, "unexpected character %q", l.ch)

	case ')':
		return l.tok1(token.RPAREN, line, col)
	case '{':
		return l.tok1(token.LBRACE, line, col)
	case '}':
		return l.tok1(token.RBRACE, line, col)
	case '[':
		return l.tok1(token.LBRACKET, line, col)
	case ']':
		return l.tok1(token.RBRACKET, line, col)
	case ',':
		return l.tok1(token.COMMA, line, col)
	case ':':
		return l.tok1(token.COLON, line, col)
	case ';':
		return l.tok1(token.SEMICOLON, line, col)
	}

	switch {
	case isDigit(l.ch):
		return l.readNumber(line, col)
	case isLetter(l.ch):
		lit := l.readIdentifier()
		return token.Token{Type: token.LookupIdent(lit), Literal: lit, Line: line, Col: col}
	default:
		ch := l.ch
		l.readChar()
		return token.Token{Type: token.ILLEGAL, Literal: fmt.Sprintf("unexpected character %q", ch), Line: line, Col: col}
	}
}

// tok1 builds a one-rune token and advances past it.
func (l *Lexer) tok1(t token.Type, line, col int) token.Token {
	tok := token.Token{Type: t, Literal: string(t), Line: line, Col: col}
	l.readChar()
	return tok
}

// tok2 builds a token whose literal was already fully consumed by the
// caller (typically after one extra readChar for a two-rune operator)
// and advances past its final rune.
func (l *Lexer) tok2(t token.Type, literal string, line, col int) token.Token {
	tok := token.Token{Type: t, Literal: literal, Line: line, Col: col}
	l.readChar()
	return tok
}

func (l *Lexer) illegal(line, col int, format string, args ...any) token.Token {
	tok := token.Token{Type: token.ILLEGAL, Literal: fmt.Sprintf(format, args...), Line: line, Col: col}
	l.readChar()
	return tok
}

// readNewline consumes the newline that triggered it, then collapses
// any further blank lines, whitespace, and comments into the same
// single NEWLINE token — so a run of blank lines in the source becomes
// one terminator, not a flood of them for the parser to skip.
func (l *Lexer) readNewline(line, col int) token.Token {
	l.readChar()
	for {
		l.skipSpacesAndTabs()
		if l.ch == '\n' {
			l.readChar()
			continue
		}
		if l.ch == '/' && l.peekChar() == '/' {
			l.skipLineComment()
			continue
		}
		break
	}
	return token.Token{Type: token.NEWLINE, Literal: "\n", Line: line, Col: col}
}

func (l *Lexer) skipSpacesAndTabs() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\r' {
		l.readChar()
	}
}

func (l *Lexer) skipLineComment() {
	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}
}

// readNumber scans an Integer or Float literal. On hitting '.' after
// the integer part, it peeks one rune further: a digit means it's a
// float's fractional part; anything else (another '.', a '<', or
// nothing) means the '.' isn't part of this number at all — it's left
// unconsumed for the next NextToken() call to lex as '..'/'.<'/ILLEGAL.
// cRust has no trailing-dot float syntax ("5." is illegal), which is
// exactly what keeps this unambiguous.
func (l *Lexer) readNumber(line, col int) token.Token {
	start := l.pos
	for isDigit(l.ch) {
		l.readChar()
	}

	isFloat := false
	if l.ch == '.' && isDigit(l.peekChar()) {
		isFloat = true
		l.readChar()
		for isDigit(l.ch) {
			l.readChar()
		}
	}

	lit := string(l.input[start:l.pos])
	if isFloat {
		return token.Token{Type: token.FLOAT, Literal: lit, Line: line, Col: col}
	}
	return token.Token{Type: token.INT, Literal: lit, Line: line, Col: col}
}

func (l *Lexer) readIdentifier() string {
	start := l.pos
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return string(l.input[start:l.pos])
}

// readString scans a double-quoted string literal, processing escape
// sequences (\n \t \r \" \\). A raw newline or EOF before the closing
// quote is an unterminated-string error rather than a multi-line
// string — cRust strings are single-line.
func (l *Lexer) readString(line, col int) token.Token {
	l.readChar() // consume opening quote

	var sb strings.Builder
	for {
		switch l.ch {
		case '"':
			l.readChar() // consume closing quote
			return token.Token{Type: token.STRING, Literal: sb.String(), Line: line, Col: col}

		case 0, '\n':
			return token.Token{Type: token.ILLEGAL, Literal: "unterminated string literal", Line: line, Col: col}

		case '\\':
			l.readChar()
			switch l.ch {
			case 'n':
				sb.WriteRune('\n')
			case 't':
				sb.WriteRune('\t')
			case 'r':
				sb.WriteRune('\r')
			case '"':
				sb.WriteRune('"')
			case '\\':
				sb.WriteRune('\\')
			case 0, '\n':
				return token.Token{Type: token.ILLEGAL, Literal: "unterminated string literal", Line: line, Col: col}
			default:
				return token.Token{Type: token.ILLEGAL, Literal: fmt.Sprintf("unknown escape sequence \\%c", l.ch), Line: line, Col: col}
			}
			l.readChar()

		default:
			sb.WriteRune(l.ch)
			l.readChar()
		}
	}
}

func isDigit(ch rune) bool {
	return unicode.IsDigit(ch)
}

func isLetter(ch rune) bool {
	return ch == '_' || unicode.IsLetter(ch)
}
