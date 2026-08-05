package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/Sintfoap/cRust/internal/debugger"
	"github.com/charmbracelet/lipgloss"
)

// Color palette: warm, muted pizza tones rather than saturated
// defaults — a KPI pie chart needs several distinct slice colors, and
// "distinct" doesn't have to mean neon. Crust (a warm tan) is reserved
// for chrome/borders, not a data slice, so it never gets confused with
// an actual KPI's own color.
var (
	colorCrust    = lipgloss.Color("180") // warm tan — borders, chrome
	colorTomato   = lipgloss.Color("167") // muted red
	colorCheese   = lipgloss.Color("179") // muted gold
	colorBasil    = lipgloss.Color("108") // muted green
	colorOlive    = lipgloss.Color("137") // muted brown-gold
	colorCrimson  = lipgloss.Color("131") // deeper red
	colorSage     = lipgloss.Color("144") // pale green
	colorMuted    = lipgloss.Color("245") // secondary text
	colorFaint    = lipgloss.Color("238") // borders, inactive chrome
	colorErrorFg  = lipgloss.Color("203")
	colorSelected = lipgloss.Color("223")
)

// sliceColors is the cycle a pie chart's wedges/legend draw from, in
// order — stable across redraws so a given KPI keeps its color as long
// as its rank doesn't change.
var sliceColors = []lipgloss.Color{colorTomato, colorCheese, colorBasil, colorOlive, colorCrimson, colorSage}

var (
	styleTitle = lipgloss.NewStyle().Bold(true).Foreground(colorCrust)
	styleMuted = lipgloss.NewStyle().Foreground(colorMuted)
	styleFaint = lipgloss.NewStyle().Foreground(colorFaint)
	styleError = lipgloss.NewStyle().Foreground(colorErrorFg)

	styleTabActive = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(colorCrust).Padding(0, 2)
	styleTabIdle   = lipgloss.NewStyle().Foreground(colorMuted).Padding(0, 2)

	styleSelectedRow = lipgloss.NewStyle().Bold(true).Foreground(colorSelected)

	styleHelp = lipgloss.NewStyle().Foreground(colorFaint)

	// styleCursor renders the Run tab's text-field cursor: reverse
	// video reads as a terminal cursor block regardless of theme,
	// without needing its own color choice.
	styleCursor = lipgloss.NewStyle().Reverse(true)
)

// legendSwatch renders one KPI's color legend entry: a colored block,
// its name, and its share — the pie chart's slices are otherwise
// unlabeled (there's no room to fit text inside a wedge at terminal
// resolution), so the legend is where a reader actually learns which
// color is which.
func legendSwatch(color lipgloss.Color, name string, pct float64, extra string) string {
	block := lipgloss.NewStyle().Foreground(color).Render("■")
	return fmt.Sprintf("%s %-22s %5.1f%%  %s", block, truncateRunes(name, 22), pct, extra)
}

// truncateRunes shortens s to at most n runes, marking truncation —
// a KPI family name is user-controlled text (a recipe/loop's own
// source), so it can be arbitrarily long.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

// pieChart renders kpis (already sorted by self time descending, as
// Timing.KPIs returns them) as a filled ASCII circle, colored by
// wedge, plus a text legend below it. share picks which KPI number
// each wedge's *size* represents — SelfTime for the time chart,
// SelfSize for the memory chart — and format renders that same KPI's
// legend text (a formatted duration for the time chart, a plain
// integer for the memory chart), so one function draws both of the
// user's requested pie charts.
//
// Terminal cells are roughly twice as tall as they are wide, so each
// logical horizontal step renders as two characters — without that
// compensation the "circle" would come out as a tall oval.
func pieChart(kpis []debugger.KPI, radius int, share func(debugger.KPI) float64, format func(debugger.KPI) string) string {
	total := 0.0
	for _, k := range kpis {
		total += share(k)
	}
	if total <= 0 {
		return styleMuted.Render("(nothing recorded yet)")
	}

	// Cumulative angle boundaries, one per KPI, in the same order
	// they're ranked — the biggest wedge starts at angle 0 (12 o'clock,
	// via the -y/x atan2 below) and they proceed clockwise in rank
	// order, so "biggest slice first, reading clockwise" always holds
	// regardless of how many KPIs there are.
	bounds := make([]float64, len(kpis)+1)
	for i, k := range kpis {
		bounds[i+1] = bounds[i] + share(k)/total
	}

	var out strings.Builder
	for ty := -radius; ty <= radius; ty++ {
		for tx := -radius; tx <= radius; tx++ {
			dist := float64(tx*tx + ty*ty)
			if dist > float64(radius*radius) {
				out.WriteString("  ")
				continue
			}
			angle := math.Atan2(float64(tx), float64(-ty)) // 0 at top, clockwise
			if angle < 0 {
				angle += 2 * math.Pi
			}
			frac := angle / (2 * math.Pi)
			idx := sliceIndex(bounds, frac)
			color := sliceColors[idx%len(sliceColors)]
			out.WriteString(lipgloss.NewStyle().Background(color).Render("  "))
		}
		out.WriteByte('\n')
	}

	out.WriteByte('\n')
	for i, k := range kpis {
		pct := 100 * share(k) / total
		extra := ""
		if format != nil {
			extra = format(k)
		}
		out.WriteString(legendSwatch(sliceColors[i%len(sliceColors)], k.Name, pct, extra))
		out.WriteByte('\n')
	}
	return out.String()
}

// sliceIndex returns which KPI (by index into bounds, 0-based) frac
// (a fraction of the full circle, [0,1)) falls into, given bounds'
// cumulative fractional boundaries (bounds[0]==0, bounds[len-1]==1).
func sliceIndex(bounds []float64, frac float64) int {
	for i := 1; i < len(bounds); i++ {
		if frac < bounds[i] {
			return i - 1
		}
	}
	return len(bounds) - 2
}
