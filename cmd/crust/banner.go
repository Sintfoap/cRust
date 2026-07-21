package main

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// pizzaArt is the plain character grid for the banner pizza. Character
// identity alone determines what gets colored as what when rendered:
// '@'/'#' is crust, 'o' is pepperoni, '*' is basil, everything else
// printable is cheese texture. Pepperoni and basil are the two
// toggleable toppings (see parseToppings) — turning one off redraws
// its characters as plain cheese instead of removing them, so the
// pizza's shape never changes, only what's on it.
const pizzaArt = `                      @
             @@@@@#########@@@@@
          @@@###:.:;..;.:;.;.###@@@
        @@##..::...ooo;:.;..;;;.;##@@
      @@##;:...;.:ooooo:.;.;:;;..;;##@@
    @@##;.:.ooo;;.;ooo.;.:;;:::;:::..##@@
   @@##;..;ooooo:;::;::;..;:ooo.:.::.;##@@
  @@##.;;::;ooo:;:;:..::;;.ooooo.;;:;;;##@@
  @##::ooo;:;:.::.;.:..:.;.:ooo::ooo..::##@
  @##;ooooo:.:;:*;::;:.....*.;..ooooo:;.##@
 @@#::.ooo.:;:;;:.;;ooo;;;;.:;;::ooo::.:;#@@
  @##:....:..:;...;ooooo.;.:;...;:.;::;:##@
  @##:..::ooo:::...;ooo:;::;.;..*;:.;;.;##@
  @@##:;.ooooo;:;:.:.;;;:;.;.ooo.:;..;:##@@
   @@##:;.ooo.:::.;*;::;::..ooooo..:.:##@@
    @@##.:*;;.:;ooo:;.;.:;.:.ooo:;:.;##@@
      @@##:::;.ooooo;...ooo..;:;.;;##@@
        @@##:;:.ooo;;..ooooo.;;.;##@@
          @@@###;.:...:.ooo:;###@@@
             @@@@@#########@@@@@
                      @`

// pizzaWidth is the longest line in pizzaArt, used to center the
// tagline underneath it.
const pizzaWidth = 44

// True-color ANSI codes, matched to the exact hex palette used in
// assets/banner.png so the terminal banner and the README banner agree.
const (
	colCrust  = "\x1b[38;2;200;138;58m" // #c88a3a
	colCheese = "\x1b[38;2;242;199;68m" // #f2c744
	colPep    = "\x1b[38;2;224;90;69m"  // #e05a45
	colBasil  = "\x1b[38;2;124;191;88m" // #7cbf58
	colReset  = "\x1b[0m"
	colBold   = "\x1b[1m"
)

const (
	toppingPepperoni = "pepperoni"
	toppingBasil     = "basil"
)

// parseToppings turns a --toppings value into the set of enabled
// toppings. "all" enables everything; "plain"/"none"/"cheese" enables
// nothing (a plain cheese pizza); otherwise it's a comma-separated list
// of topping names.
func parseToppings(spec string) (map[string]bool, error) {
	switch strings.TrimSpace(spec) {
	case "all":
		return map[string]bool{toppingPepperoni: true, toppingBasil: true}, nil
	case "plain", "none", "cheese":
		return map[string]bool{}, nil
	}

	enabled := map[string]bool{}
	for _, t := range strings.Split(spec, ",") {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		switch t {
		case toppingPepperoni, toppingBasil:
			enabled[t] = true
		default:
			return nil, fmt.Errorf("unknown topping %q — pick from: pepperoni, basil, all, plain", t)
		}
	}
	return enabled, nil
}

// renderPizza draws pizzaArt, colorizing it (or not) according to
// color, and swapping any disabled topping's characters for plain
// cheese instead of its usual glyph and color.
func renderPizza(toppings map[string]bool, color bool) string {
	var b strings.Builder
	for _, line := range strings.Split(pizzaArt, "\n") {
		current := ""
		for _, r := range line {
			code, out := "", r
			switch {
			case r == '@' || r == '#':
				code = colCrust
			case r == 'o':
				if toppings[toppingPepperoni] {
					code = colPep
				} else {
					code, out = colCheese, '.'
				}
			case r == '*':
				if toppings[toppingBasil] {
					code = colBasil
				} else {
					code, out = colCheese, '.'
				}
			case r == ' ':
				// no color
			default:
				code = colCheese
			}

			if color && code != current {
				if current != "" {
					b.WriteString(colReset)
				}
				if code != "" {
					b.WriteString(code)
				}
				current = code
			}
			b.WriteRune(out)
		}
		if color && current != "" {
			b.WriteString(colReset)
		}
		b.WriteRune('\n')
	}
	return b.String()
}

// banner renders the pizza plus a centered tagline underneath it.
func banner(toppings map[string]bool, color bool) string {
	var b strings.Builder
	b.WriteString(renderPizza(toppings, color))
	b.WriteString(centeredTagline(color))
	b.WriteString("\n\n")
	return b.String()
}

func centeredTagline(color bool) string {
	const plain = "cRust — language baked better"
	pad := (pizzaWidth - utf8.RuneCountInString(plain)) / 2
	if pad < 0 {
		pad = 0
	}
	text := plain
	if color {
		text = colBold + colCheese + plain + colReset
	}
	return strings.Repeat(" ", pad) + text
}
