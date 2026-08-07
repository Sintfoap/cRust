package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// writeCrustFileIn writes a .crust file named name into dir with the
// given content, returning its full path — unlike writeDebugFile
// (which always gets its own fresh, isolated t.TempDir()), this lets a
// test put several files side by side in one directory, which
// listCrustFiles/the Files tab needs to have anything to find.
func writeCrustFileIn(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestListCrustFilesFindsSiblings(t *testing.T) {
	dir := t.TempDir()
	writeCrustFileIn(t, dir, "day01.crust", "x = 1")
	day02 := writeCrustFileIn(t, dir, "day02.crust", "x = 2")
	writeCrustFileIn(t, dir, "notes.txt", "not a crust file")

	files := listCrustFiles(day02)
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2 (day01.crust, day02.crust); files = %v", len(files), files)
	}
	if filepath.Base(files[0]) != "day01.crust" || filepath.Base(files[1]) != "day02.crust" {
		t.Errorf("files = %v, want day01.crust and day02.crust only", files)
	}
}

func TestListCrustFilesSortsAlphabetically(t *testing.T) {
	dir := t.TempDir()
	writeCrustFileIn(t, dir, "day10.crust", "")
	writeCrustFileIn(t, dir, "day02.crust", "")
	day01 := writeCrustFileIn(t, dir, "day01.crust", "")

	files := listCrustFiles(day01)
	var names []string
	for _, f := range files {
		names = append(names, filepath.Base(f))
	}
	want := []string{"day01.crust", "day02.crust", "day10.crust"}
	if len(names) != len(want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("names = %v, want %v", names, want)
			break
		}
	}
}

func TestListCrustFilesAloneReturnsJustItself(t *testing.T) {
	path := writeDebugFile(t, "x = 1")
	files := listCrustFiles(path)
	if len(files) != 1 || files[0] != path {
		t.Errorf("listCrustFiles(%q) = %v, want [%q]", path, files, path)
	}
}

func TestListCrustFilesMissingDirDegradesToJustCurrent(t *testing.T) {
	missing := "/no/such/directory/day01.crust"
	files := listCrustFiles(missing)
	if len(files) != 1 || files[0] != missing {
		t.Errorf("listCrustFiles(%q) = %v, want [%q]", missing, files, missing)
	}
}

func TestIndexOfNavFileFound(t *testing.T) {
	files := []string{"a/day01.crust", "a/day02.crust"}
	if got := indexOfNavFile(files, "a/day02.crust"); got != 1 {
		t.Errorf("indexOfNavFile = %d, want 1", got)
	}
}

func TestIndexOfNavFileNotFoundDefaultsToZero(t *testing.T) {
	files := []string{"a/day01.crust"}
	if got := indexOfNavFile(files, "a/nope.crust"); got != 0 {
		t.Errorf("indexOfNavFile = %d, want 0", got)
	}
}

func TestRefreshNavFilesPositionsCursorOnCurrentFile(t *testing.T) {
	dir := t.TempDir()
	writeCrustFileIn(t, dir, "day01.crust", "x = 1")
	day02 := writeCrustFileIn(t, dir, "day02.crust", "x = 2")

	m := newDebugModel(&debugView{path: day02, rec: viewFor(t, "x = 1").rec})
	m.refreshNavFiles()

	if len(m.navFiles) != 2 {
		t.Fatalf("got %d navFiles, want 2", len(m.navFiles))
	}
	if m.navCursor != 1 {
		t.Errorf("navCursor = %d, want 1 (day02.crust's position)", m.navCursor)
	}
}

func TestRefreshNavFilesClearsAPreviousError(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.navErr = "boom"
	m.refreshNavFiles()
	if m.navErr != "" {
		t.Errorf("navErr = %q, want cleared by refreshNavFiles", m.navErr)
	}
}

func TestMoveNavCursorClampsAtBothEnds(t *testing.T) {
	m := debugModel{navFiles: []string{"a", "b", "c"}, navCursor: 0}
	m.moveNavCursor(-1)
	if m.navCursor != 0 {
		t.Errorf("navCursor = %d, want 0 (clamped, not wrapped)", m.navCursor)
	}
	m.moveNavCursor(1)
	m.moveNavCursor(1)
	m.moveNavCursor(1)
	if m.navCursor != 2 {
		t.Errorf("navCursor = %d, want 2 (clamped at the last index)", m.navCursor)
	}
}

func TestMoveNavCursorNoFilesIsNoOp(t *testing.T) {
	m := debugModel{}
	m.moveNavCursor(1)
	if m.navCursor != 0 {
		t.Errorf("navCursor = %d, want 0", m.navCursor)
	}
}

func TestSwitchToSelectedFileSwitchesToTargetFile(t *testing.T) {
	dir := t.TempDir()
	writeCrustFileIn(t, dir, "day01.crust", `deliver("one")`)
	day02 := writeCrustFileIn(t, dir, "day02.crust", `deliver("two")`)

	m := newDebugModel(&debugView{path: day02, rec: viewFor(t, "x = 1").rec})
	m.refreshNavFiles()
	m.navCursor = 0 // day01.crust

	got := m.switchToSelectedFile()
	if filepath.Base(got.view.path) != "day01.crust" {
		t.Errorf("view.path = %q, want day01.crust", got.view.path)
	}
	if got.active != tabTime {
		t.Errorf("active = %v, want tabTime after switching", got.active)
	}
}

func TestSwitchToSelectedFileOnCurrentFileIsNoOp(t *testing.T) {
	dir := t.TempDir()
	day01 := writeCrustFileIn(t, dir, "day01.crust", `deliver("one")`)
	writeCrustFileIn(t, dir, "day02.crust", `deliver("two")`)

	original := viewFor(t, "x = 1").rec
	m := newDebugModel(&debugView{path: day01, rec: original})
	m.refreshNavFiles() // navCursor lands on day01.crust, the current file

	got := m.switchToSelectedFile()
	if got.view.rec != original {
		t.Error("expected the existing recording to be left alone for a no-op switch")
	}
}

func TestSwitchToSelectedFileNoFilesIsNoOp(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	got := m.switchToSelectedFile()
	if got.view != m.view {
		t.Error("expected no change when navFiles is empty")
	}
}

func TestSwitchToFileParseErrorShowsNavErrAndKeepsCurrentView(t *testing.T) {
	dir := t.TempDir()
	day01 := writeCrustFileIn(t, dir, "day01.crust", `deliver("one")`)
	writeCrustFileIn(t, dir, "day02.crust", "x = (\n") // unparseable

	original := viewFor(t, "x = 1").rec
	m := newDebugModel(&debugView{path: day01, rec: original})

	got := m.switchToFile(filepath.Join(dir, "day02.crust"))
	if got.navErr == "" {
		t.Error("expected navErr to be set for a parse error")
	}
	if got.view.rec != original {
		t.Error("expected the current (still-valid) view to be left alone on failure")
	}
}

func TestSwitchToFileRestoresPerFileSettings(t *testing.T) {
	withTempDevelStateDir(t)
	dir := t.TempDir()
	day02 := writeCrustFileIn(t, dir, "day02.crust", `
recipe store_part1() { deliver("p1") }
recipe store_part2() { deliver("p2") }`)
	saveDevelStateBestEffort(day02, "part2", "input.txt", true)

	m := newDebugModel(&debugView{path: writeDebugFile(t, "x = 1"), rec: viewFor(t, "x = 1").rec})
	got := m.switchToFile(day02)

	if got.opts.Store != "part2" {
		t.Errorf("opts.Store = %q, want %q", got.opts.Store, "part2")
	}
	if got.runInput.String() != "input.txt" {
		t.Errorf("runInput = %q, want %q", got.runInput.String(), "input.txt")
	}
	if !got.runAllStores {
		t.Error("expected runAllStores restored from the target file's saved state")
	}
	if got.runEntryIndex != 1 {
		t.Errorf("runEntryIndex = %d, want 1 (part2)", got.runEntryIndex)
	}
}

func TestSwitchToFileResetsStepperAndRunOutputState(t *testing.T) {
	dir := t.TempDir()
	day01 := writeCrustFileIn(t, dir, "day01.crust", `deliver("one")`)
	day02 := writeCrustFileIn(t, dir, "day02.crust", `deliver("two")`)

	m := newDebugModel(&debugView{path: day01, rec: viewFor(t, "x = 1").rec})
	m.runOutput = "stale output"
	m.runFailed = true
	m.cursor, m.top = 5, 2

	got := m.switchToFile(day02)
	if got.runOutput != "" || got.runFailed {
		t.Errorf("runOutput/runFailed = %q/%v, want cleared", got.runOutput, got.runFailed)
	}
	if got.cursor != 0 || got.top != 0 {
		t.Errorf("cursor/top = %d/%d, want reset to 0/0", got.cursor, got.top)
	}
}

func TestViewNavListsFilesWithCurrentMarked(t *testing.T) {
	dir := t.TempDir()
	day01 := writeCrustFileIn(t, dir, "day01.crust", "x = 1")
	writeCrustFileIn(t, dir, "day02.crust", "x = 2")

	m := newDebugModel(&debugView{path: day01, rec: viewFor(t, "x = 1").rec})
	m.refreshNavFiles()
	out := m.viewNav()

	if !strings.Contains(out, "day01.crust") || !strings.Contains(out, "day02.crust") {
		t.Errorf("viewNav() = %q, want both files listed", out)
	}
	if !strings.Contains(out, "(current)") {
		t.Errorf("viewNav() = %q, want the current file marked", out)
	}
}

func TestViewNavShowsPlaceholderWithNoSiblings(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.refreshNavFiles()
	out := m.viewNav()
	if !strings.Contains(out, "no other") {
		t.Errorf("viewNav() = %q, want a no-other-files placeholder", out)
	}
}

func TestViewNavShowsErrorAboveList(t *testing.T) {
	dir := t.TempDir()
	day01 := writeCrustFileIn(t, dir, "day01.crust", "x = 1")
	writeCrustFileIn(t, dir, "day02.crust", "x = 2")

	m := newDebugModel(&debugView{path: day01, rec: viewFor(t, "x = 1").rec})
	m.refreshNavFiles()
	m.navErr = "boom"
	out := m.viewNav()
	if !strings.Contains(out, "boom") {
		t.Errorf("viewNav() = %q, want the error shown", out)
	}
	if !strings.Contains(out, "day02.crust") {
		t.Errorf("viewNav() = %q, want the file list still shown alongside the error", out)
	}
}

// --- key-handling / integration ---------------------------------------

func TestHandleKeyEnterOnFilesTabSwitches(t *testing.T) {
	dir := t.TempDir()
	day01 := writeCrustFileIn(t, dir, "day01.crust", `deliver("one")`)
	day02 := writeCrustFileIn(t, dir, "day02.crust", `deliver("two")`)

	m := newDebugModel(&debugView{path: day02, rec: viewFor(t, "x = 1").rec})
	m.active = tabNav
	m.refreshNavFiles()
	m.navCursor = indexOfNavFile(m.navFiles, day01)

	next, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(debugModel)
	if filepath.Base(got.view.path) != "day01.crust" {
		t.Errorf("view.path = %q, want day01.crust", got.view.path)
	}
}

func TestHandleKeyUpDownMovesNavCursor(t *testing.T) {
	dir := t.TempDir()
	writeCrustFileIn(t, dir, "day01.crust", "x = 1")
	day02 := writeCrustFileIn(t, dir, "day02.crust", "x = 2")

	m := newDebugModel(&debugView{path: day02, rec: viewFor(t, "x = 1").rec})
	m.active = tabNav
	m.refreshNavFiles() // cursor starts on day02.crust (index 1)

	next, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyUp})
	got := next.(debugModel)
	if got.navCursor != 0 {
		t.Errorf("navCursor = %d, want 0 after up", got.navCursor)
	}
}

func TestTabCyclingFromTimeReachesFilesTab(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	for i := 0; i < 5; i++ {
		next, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
		m = next.(debugModel)
	}
	if m.active != tabNav {
		t.Errorf("active = %v after 5 tabs from Time, want tabNav", m.active)
	}
}

// TestTabFromRunReachesFilesTabAndRefreshesIt exercises the specific
// path that broke on first pass: reaching Files by tabbing *forward*
// from Run goes through handleRunTabKey, not the shared handleKey
// switch, so that handler needs its own maybeRefreshNav call too.
func TestTabFromRunReachesFilesTabAndRefreshesIt(t *testing.T) {
	dir := t.TempDir()
	writeCrustFileIn(t, dir, "day01.crust", "x = 1")
	day02 := writeCrustFileIn(t, dir, "day02.crust", "x = 2")

	m := newDebugModel(&debugView{path: day02, rec: viewFor(t, "x = 1").rec})
	m.active = tabRun

	next, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	got := next.(debugModel)
	if got.active != tabNav {
		t.Fatalf("active = %v, want tabNav", got.active)
	}
	if len(got.navFiles) != 2 {
		t.Errorf("got %d navFiles, want 2 (refreshed on entering the tab)", len(got.navFiles))
	}
}

func TestShiftTabFromFilesReturnsToRun(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabNav
	next, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyShiftTab})
	got := next.(debugModel)
	if got.active != tabRun {
		t.Errorf("active = %v, want tabRun", got.active)
	}
}

func TestMaybeRefreshNavOnlyActsOnFilesTab(t *testing.T) {
	dir := t.TempDir()
	writeCrustFileIn(t, dir, "day01.crust", "x = 1")
	day02 := writeCrustFileIn(t, dir, "day02.crust", "x = 2")

	m := newDebugModel(&debugView{path: day02, rec: viewFor(t, "x = 1").rec})
	m.active = tabTime
	m.maybeRefreshNav()
	if m.navFiles != nil {
		t.Error("expected maybeRefreshNav to be a no-op off the Files tab")
	}

	m.active = tabNav
	m.maybeRefreshNav()
	if len(m.navFiles) != 2 {
		t.Errorf("got %d navFiles after entering the tab, want 2", len(m.navFiles))
	}
}

func TestViewTabsIncludesFilesLabel(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	out := m.viewTabs()
	if !strings.Contains(out, "Files") {
		t.Errorf("viewTabs() = %q, want a Files tab label", out)
	}
}

func TestHelpTextOnFilesTab(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabNav
	if !strings.Contains(m.helpText(), "switch") {
		t.Errorf("helpText() = %q, want it to mention switching", m.helpText())
	}
}

func TestViewRendersFilesTabWithoutPanicking(t *testing.T) {
	dir := t.TempDir()
	day01 := writeCrustFileIn(t, dir, "day01.crust", "x = 1")
	writeCrustFileIn(t, dir, "day02.crust", "x = 2")

	m := newDebugModel(&debugView{path: day01, rec: viewFor(t, "x = 1").rec})
	m.width, m.height = 80, 24
	m.active = tabNav
	m.refreshNavFiles()
	out := m.View()
	if !strings.Contains(out, "day02.crust") {
		t.Errorf("View() = %q, want the Files tab body rendered", out)
	}
}
