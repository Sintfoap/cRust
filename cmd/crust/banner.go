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
// wordmark and tagline underneath it.
const pizzaWidth = 44

// crustLogo is the "cRust" wordmark, rendered with the ansi_shadow
// figlet font (pyfiglet: Figlet(font="ansi_shadow").renderText("cRust")) —
// the same font the reference CLI's own banner uses. figlet block fonts
// don't distinguish case for glyphs this size, so it comes out as
// CRUST; matching the reference's all-caps convention read better here
// than hand-forcing a smaller "c" into an otherwise uniform block font.
const crustLogo = ` ██████╗██████╗ ██╗   ██╗███████╗████████╗
██╔════╝██╔══██╗██║   ██║██╔════╝╚══██╔══╝
██║     ██████╔╝██║   ██║███████╗   ██║
██║     ██╔══██╗██║   ██║╚════██║   ██║
╚██████╗██║  ██║╚██████╔╝███████║   ██║
 ╚═════╝╚═╝  ╚═╝ ╚═════╝ ╚══════╝   ╚═╝`

const logoWidth = 42

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

// banner renders the pizza, the "cRust" wordmark, and the tagline,
// each centered to pizzaWidth.
func banner(toppings map[string]bool, color bool) string {
	var b strings.Builder
	b.WriteString(renderPizza(toppings, color))
	b.WriteString(centeredBlock(crustLogo, logoWidth, colBold+colCheese, color))
	b.WriteString("\n")
	b.WriteString(centeredLine("language baked better", colBold+colPep, color))
	b.WriteString("\n\n")
	return b.String()
}

// centeredBlock centers a multi-line block within pizzaWidth, coloring
// each line individually (rather than spanning one escape across
// newlines, which not every terminal/pager handles cleanly).
func centeredBlock(block string, blockWidth int, code string, color bool) string {
	pad := (pizzaWidth - blockWidth) / 2
	if pad < 0 {
		pad = 0
	}
	prefix := strings.Repeat(" ", pad)

	var b strings.Builder
	for _, line := range strings.Split(block, "\n") {
		b.WriteString(prefix)
		if color {
			b.WriteString(code)
			b.WriteString(line)
			b.WriteString(colReset)
		} else {
			b.WriteString(line)
		}
		b.WriteRune('\n')
	}
	return b.String()
}

func centeredLine(text, code string, color bool) string {
	pad := (pizzaWidth - utf8.RuneCountInString(text)) / 2
	if pad < 0 {
		pad = 0
	}
	out := text
	if color {
		out = code + text + colReset
	}
	return strings.Repeat(" ", pad) + out
}
