package main

import "io"

// runDebugTUI will drive the interactive two-tab stepper (KPI pie
// charts + tree stepper) on a real terminal. Not built yet — falls
// back to the plain-text view in the meantime so `crust debug` is
// fully usable (just not interactively) while that lands.
func runDebugTUI(view *debugView, stdin io.Reader, stdout, stderr io.Writer) int {
	view.writePlain(stdout)
	return 0
}
