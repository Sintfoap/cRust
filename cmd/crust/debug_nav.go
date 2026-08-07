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

	tea "github.com/charmbracelet/bubbletea"
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

// handleNavCreateKey routes keys while the Files tab's "new file"
// prompt is up (m.navCreating) — reusing runInputModel's own key
// handling (insert/backspace/left/right), the same field the Run tab's
// input-file box already uses, plus the three keys the field itself
// has no use for: Enter to actually create the file, Esc to back out
// to the plain file list without creating anything, and Ctrl+C as the
// same always-available hard quit every other tab keeps regardless of
// what's mid-edit (the Run tab's own input field makes the identical
// choice, and for the identical reason — see handleRunTabKey's doc
// comment). Esc deliberately does *not* quit here, unlike the Run
// tab's field: that field is a permanent fixture of its tab with
// nothing to "cancel" back out of, while this one is a transient
// prompt over the ordinary Files list, so Esc reads as "close this
// prompt" the way it would for any other modal input.
func (m debugModel) handleNavCreateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.navCreating = false
		m.navNewName = runInputModel{}
		m.navErr = ""
	case tea.KeyCtrlC:
		return m, tea.Quit
	case tea.KeyEnter:
		return m.createNavFile()
	case tea.KeyLeft:
		m.navNewName.left()
	case tea.KeyRight:
		m.navNewName.right()
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			m.navNewName.insert(r)
		}
	case tea.KeySpace:
		m.navNewName.insert(' ')
	case tea.KeyBackspace:
		m.navNewName.backspace()
	case tea.KeyDelete:
		m.navNewName.deleteForward()
	case tea.KeyHome, tea.KeyCtrlA:
		m.navNewName.cursor = 0
	case tea.KeyEnd, tea.KeyCtrlE:
		m.navNewName.cursor = len(m.navNewName.value)
	}
	return m, nil
}

// createNavFile creates a new, empty .crust file alongside the one
// currently open (the same directory listCrustFiles already scopes
// to) from whatever name was typed into the Files tab's prompt, and
// switches straight to it — the interactive counterpart to `crust
// develop newday.crust` auto-creating a missing target file
// (debug.go's ensureFileExists), for starting a new day's file without
// leaving the session at all. A ".crust" suffix is appended
// automatically when the typed name doesn't already end in one, since
// typing the extension every single time is exactly the friction this
// exists to remove.
//
// Unlike ensureFileExists (which treats "the file's already there" as
// success, since a missing target is the expected case it's guarding
// against), a name that already exists here is reported as navErr
// instead: this is an explicit "make something new" action, not an
// idempotent auto-create, so silently switching to someone's existing
// day01.crust after they typed its name by habit would be a surprise,
// not a convenience. O_CREATE|O_EXCL keeps the existence check and the
// create atomic, same reasoning as ensureFileExists. Any failure
// (empty name, already exists, unwritable directory) leaves navCreating
// on so the prompt stays up to fix and retry, rather than silently
// dropping back to the plain list.
func (m debugModel) createNavFile() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.navNewName.String())
	if name == "" {
		m.navErr = "type a filename first"
		return m, nil
	}
	if !strings.HasSuffix(name, ".crust") {
		name += ".crust"
	}
	path := filepath.Join(filepath.Dir(m.view.path), name)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			m.navErr = name + " already exists"
		} else {
			m.navErr = err.Error()
		}
		return m, nil
	}
	f.Close()

	m.navCreating = false
	m.navNewName = runInputModel{}
	return m.switchToFile(path), nil
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
//
// The "new file" prompt (m.navCreating) takes over the whole tab
// rather than appearing alongside the list — checked first, ahead of
// even the "only one file here" early return below, since starting a
// second file in an otherwise-empty directory is exactly the case this
// exists for.
func (m debugModel) viewNav() string {
	if m.navCreating {
		var b strings.Builder
		b.WriteString(styleTitle.Render("new file name (created alongside " + filepath.Base(m.view.path) + "):"))
		b.WriteByte('\n')
		b.WriteString("  " + m.navNewName.render())
		if m.navErr != "" {
			b.WriteString("\n\n")
			b.WriteString(styleError.Render(m.navErr))
		}
		return b.String()
	}

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
