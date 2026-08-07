// The Files tab (debug_tui.go's tabNav): browse and switch to another
// .crust file alongside the one currently being debugged, without
// leaving `crust develop` and restarting it against a different path
// on the command line — the natural next thing to want on a directory
// full of day01.crust..day25.crust-style AoC solutions.
package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Sintfoap/cRust/internal/debugger"
)

// listCrustFiles returns every *.crust file (including currentPath
// itself) in currentPath's own directory, sorted alphabetically — the
// natural order day01.crust..day25.crust-style AoC filenames already
// sort into, so there's no need for anything fancier. A directory-read
// failure (shouldn't happen for a file that's already open, but a
// filesystem is never fully trustworthy) degrades to just
// []string{currentPath} rather than erroring, the same "don't fail the
// whole tab over a listing problem" choice scanEntryPoints already
// makes for a bad parse.
func listCrustFiles(currentPath string) []string {
	dir := filepath.Dir(currentPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{currentPath}
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".crust") {
			continue
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	if len(files) == 0 {
		return []string{currentPath}
	}
	sort.Strings(files)
	return files
}

// indexOfNavFile returns current's position among files (compared by
// base name, not the full path string — listCrustFiles always joins
// against the same directory current came from, so a base-name match
// is unambiguous and immune to current being spelled with a leading
// "./" or not), or 0 if it isn't found at all (shouldn't happen, since
// listCrustFiles always includes current, but 0 is a safe fallback the
// same way indexOfEntry's is).
func indexOfNavFile(files []string, current string) int {
	base := filepath.Base(current)
	for i, f := range files {
		if filepath.Base(f) == base {
			return i
		}
	}
	return 0
}

// refreshNavFiles rescans m.view.path's directory and repositions
// navCursor onto whichever entry is the file currently open. Called
// whenever a tab switch lands on Files (maybeRefreshNav) rather than
// cached across the session — directory contents can change between
// visits (a file added, renamed, or removed), and an os.ReadDir is
// cheap enough that there's nothing to gain by trusting a stale list.
// Also clears any error left over from a previous visit's failed
// switch attempt — a fresh look at the tab is a fresh start, not a
// reason to keep showing a problem from before.
func (m *debugModel) refreshNavFiles() {
	m.navFiles = listCrustFiles(m.view.path)
	m.navCursor = indexOfNavFile(m.navFiles, m.view.path)
	m.navErr = ""
}

// maybeRefreshNav rescans the directory listing the moment a tab
// switch lands on Files, mirroring maybeOpenEditor's "switching to the
// tab is the action" shape for the Editor tab — a no-op for every
// other tab, so tab/shift-tab stays free elsewhere exactly as before
// this existed.
func (m *debugModel) maybeRefreshNav() {
	if m.active != tabNav {
		return
	}
	m.refreshNavFiles()
}

// moveNavCursor moves navCursor by delta, clamped at both ends (not
// wrapping) — a small, usually-short list of files where "past the
// last one" landing back on the first would be more surprising than
// useful, unlike the tab bar's own intentional wraparound.
func (m *debugModel) moveNavCursor(delta int) {
	if len(m.navFiles) == 0 {
		return
	}
	m.navCursor += delta
	if m.navCursor < 0 {
		m.navCursor = 0
	}
	if m.navCursor >= len(m.navFiles) {
		m.navCursor = len(m.navFiles) - 1
	}
}

// switchToSelectedFile switches to whichever file navCursor currently
// points at, unless it's the file already open — a redundant enter on
// the current row would otherwise wipe out an existing recording for
// nothing, since switchToFile always starts from a fresh, empty view.
func (m debugModel) switchToSelectedFile() debugModel {
	if len(m.navFiles) == 0 {
		return m
	}
	target := m.navFiles[m.navCursor]
	if filepath.Base(target) == filepath.Base(m.view.path) {
		return m
	}
	return m.switchToFile(target)
}

// switchToFile replaces m.view with a fresh, empty (unrecorded) view
// of path — the same "parsed but not yet run" starting point the whole
// session begins with (runDebug's real-terminal path builds an
// emptyDebugView before bubbletea ever takes the screen, so that a
// store recipe's unbox() can never silently eat a keystroke meant for
// the TUI). Every per-file setting (opts.Store, the Run tab's input
// path and run-all toggle) is reloaded from that new file's own
// remembered state (debug_state.go) rather than carried over from the
// file being left, the same way a fresh `crust develop otherday.crust`
// invocation would start. A parse failure in the target file is shown
// on the Files tab (navErr) without disturbing the still-valid current
// view — the same "leave the last good state alone" choice
// handleReload already makes for a save that breaks the file.
func (m debugModel) switchToFile(path string) debugModel {
	view, err := emptyDebugView(path)
	if err != nil {
		m.navErr = err.Error()
		return m
	}

	m.navErr = ""
	m.editorErr = ""
	m.view = view
	m.expanded = map[*debugger.TraceNode]bool{}
	m.cursor, m.top = 0, 0
	m.rebuildRows()

	m.opts = applySavedStore(path, debugOptions{MaxSteps: m.opts.MaxSteps})
	m.runEntryIndex = indexOfEntry(view.entryPoints(), m.opts.Store)
	m.runEntryFocused = false
	m.runInput = restoreRunInput(path)
	m.runAllStores = restoreRunAll(path)
	m.runOutput = ""
	m.runFailed = false

	m.active = tabTime
	return m
}

// viewNav lists every file listCrustFiles found alongside the one
// currently open, one per line: "> " marks navCursor (enter's target,
// the same cursor-row convention the Stepper and Run tab's own
// selector already use), and "(current)" marks whichever row is the
// file actually open right now — worth telling apart from navCursor,
// since they start in the same place but don't have to stay there. A
// pending navErr renders above the list rather than in place of it —
// showing only the error and hiding every file would leave no way to
// pick a different, working target after a failed switch attempt.
func (m debugModel) viewNav() string {
	if len(m.navFiles) <= 1 {
		msg := "no other .crust files found alongside " + filepath.Base(m.view.path)
		if m.navErr != "" {
			msg = m.navErr
		}
		return styleMuted.Render(msg)
	}

	var b strings.Builder
	if m.navErr != "" {
		b.WriteString(styleError.Render(m.navErr))
		b.WriteString("\n\n")
	}
	for i, f := range m.navFiles {
		marker := "  "
		style := lipgloss.NewStyle()
		if i == m.navCursor {
			marker = "> "
			style = styleSelectedRow
		}
		label := filepath.Base(f)
		if filepath.Base(f) == filepath.Base(m.view.path) {
			label += " (current)"
		}
		b.WriteString(marker + style.Render(label))
		b.WriteByte('\n')
	}
	return b.String()
}
