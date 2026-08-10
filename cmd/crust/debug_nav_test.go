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

// --- delivery-usage hints -------------------------------------------------

func TestDeliveryTargetsFindsTopLevelDeliveryStatements(t *testing.T) {
	dir := t.TempDir()
	day01 := writeCrustFileIn(t, dir, "day01.crust", `delivery "grid_utils.crust"
delivery "parsing.crust"
deliver("hi")`)

	got := deliveryTargets(day01)
	want := []string{"grid_utils.crust", "parsing.crust"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
			break
		}
	}
}

func TestDeliveryTargetsNoneReturnsNil(t *testing.T) {
	path := writeDebugFile(t, `deliver("hi")`)
	got := deliveryTargets(path)
	if len(got) != 0 {
		t.Errorf("got %v, want none", got)
	}
}

func TestDeliveryTargetsParseErrorReturnsNil(t *testing.T) {
	path := writeDebugFile(t, "x = (\n")
	got := deliveryTargets(path)
	if len(got) != 0 {
		t.Errorf("got %v, want nil for an unparseable file", got)
	}
}

func TestDeliveryTargetsMissingFileReturnsNil(t *testing.T) {
	got := deliveryTargets("/no/such/file.crust")
	if len(got) != 0 {
		t.Errorf("got %v, want nil", got)
	}
}

func TestComputeDeliveryUsageCountsAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	writeCrustFileIn(t, dir, "grid_utils.crust", `recipe manhattan(a, b) { serve 0 }`)
	day01 := writeCrustFileIn(t, dir, "day01.crust", `delivery "grid_utils.crust"`)
	day02 := writeCrustFileIn(t, dir, "day02.crust", `delivery "grid_utils.crust"`)
	day03 := writeCrustFileIn(t, dir, "day03.crust", `deliver("no imports here")`)

	usage := computeDeliveryUsage([]string{day01, day02, day03, filepath.Join(dir, "grid_utils.crust")})
	if usage["grid_utils.crust"] != 2 {
		t.Errorf("usage[grid_utils.crust] = %d, want 2", usage["grid_utils.crust"])
	}
	if usage["day01.crust"] != 0 {
		t.Errorf("usage[day01.crust] = %d, want 0 (nothing delivers it)", usage["day01.crust"])
	}
}

func TestComputeDeliveryUsageExcludesSelfDelivery(t *testing.T) {
	dir := t.TempDir()
	self := writeCrustFileIn(t, dir, "weird.crust", `delivery "weird.crust"`)

	usage := computeDeliveryUsage([]string{self})
	if usage["weird.crust"] != 0 {
		t.Errorf("usage[weird.crust] = %d, want 0 (a file delivering itself shouldn't count)", usage["weird.crust"])
	}
}

func TestRefreshNavFilesPopulatesNavUsage(t *testing.T) {
	dir := t.TempDir()
	writeCrustFileIn(t, dir, "grid_utils.crust", "")
	day01 := writeCrustFileIn(t, dir, "day01.crust", `delivery "grid_utils.crust"`)

	m := newDebugModel(&debugView{path: day01, rec: viewFor(t, "x = 1").rec})
	m.refreshNavFiles()

	if m.navUsage["grid_utils.crust"] != 1 {
		t.Errorf("navUsage[grid_utils.crust] = %d, want 1", m.navUsage["grid_utils.crust"])
	}
}

func TestViewNavShowsUsageHint(t *testing.T) {
	dir := t.TempDir()
	writeCrustFileIn(t, dir, "grid_utils.crust", "")
	day01 := writeCrustFileIn(t, dir, "day01.crust", `delivery "grid_utils.crust"`)
	writeCrustFileIn(t, dir, "day02.crust", `delivery "grid_utils.crust"`)

	m := newDebugModel(&debugView{path: day01, rec: viewFor(t, "x = 1").rec})
	m.refreshNavFiles()
	out := m.viewNav()

	if !strings.Contains(out, "used by 2 other files") {
		t.Errorf("viewNav() = %q, want a used-by-2 hint for grid_utils.crust", out)
	}
}

func TestViewNavUsageHintSingularWording(t *testing.T) {
	dir := t.TempDir()
	writeCrustFileIn(t, dir, "grid_utils.crust", "")
	day01 := writeCrustFileIn(t, dir, "day01.crust", `delivery "grid_utils.crust"`)

	m := newDebugModel(&debugView{path: day01, rec: viewFor(t, "x = 1").rec})
	m.refreshNavFiles()
	out := m.viewNav()

	if !strings.Contains(out, "used by 1 other file)") {
		t.Errorf("viewNav() = %q, want singular 'file' wording for a count of 1", out)
	}
}

func TestViewNavNoUsageHintWhenNeverDelivered(t *testing.T) {
	dir := t.TempDir()
	day01 := writeCrustFileIn(t, dir, "day01.crust", "x = 1")
	writeCrustFileIn(t, dir, "day02.crust", "x = 2")

	m := newDebugModel(&debugView{path: day01, rec: viewFor(t, "x = 1").rec})
	m.refreshNavFiles()
	out := m.viewNav()

	if strings.Contains(out, "used by") {
		t.Errorf("viewNav() = %q, want no usage hint when nothing delivers anything", out)
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
	for i := 0; i < 6; i++ {
		next, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
		m = next.(debugModel)
	}
	if m.active != tabNav {
		t.Errorf("active = %v after 6 tabs from Time, want tabNav", m.active)
	}
}

// TestTabFromRunReachesFilesTabAndRefreshesIt exercises the specific
// path that broke on first pass: reaching Files by tabbing *forward*
// from Run (through Bench, which sits between them) goes through
// handleRunTabKey then handleBenchTabKey, neither of them the shared
// handleKey switch, so both need their own maybeRefreshNav call too.
func TestTabFromRunReachesFilesTabAndRefreshesIt(t *testing.T) {
	dir := t.TempDir()
	writeCrustFileIn(t, dir, "day01.crust", "x = 1")
	day02 := writeCrustFileIn(t, dir, "day02.crust", "x = 2")

	m := newDebugModel(&debugView{path: day02, rec: viewFor(t, "x = 1").rec})
	m.active = tabRun

	next, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	got := next.(debugModel)
	if got.active != tabBench {
		t.Fatalf("active = %v, want tabBench (Run's own next tab)", got.active)
	}

	next2, _ := got.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	got2 := next2.(debugModel)
	if got2.active != tabNav {
		t.Fatalf("active = %v, want tabNav", got2.active)
	}
	if len(got2.navFiles) != 2 {
		t.Errorf("got %d navFiles, want 2 (refreshed on entering the tab)", len(got2.navFiles))
	}
}

func TestShiftTabFromFilesReturnsToBench(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabNav
	next, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyShiftTab})
	got := next.(debugModel)
	if got.active != tabBench {
		t.Errorf("active = %v, want tabBench", got.active)
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

// --- create-new-file ----------------------------------------------------

func TestKeyNOnFilesTabStartsCreating(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabNav

	next, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	got := next.(debugModel)
	if !got.navCreating {
		t.Error("expected navCreating to be true after pressing n on the Files tab")
	}
}

func TestKeyNElsewhereIsNoOp(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabTime

	next, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	got := next.(debugModel)
	if got.navCreating {
		t.Error("expected n to be a no-op off the Files tab")
	}
}

func TestHandleNavCreateKeyTypesIntoNewName(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.navCreating = true

	next, _ := m.handleNavCreateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("day06")})
	got := next.(debugModel)
	if got.navNewName.String() != "day06" {
		t.Errorf("navNewName = %q, want %q", got.navNewName.String(), "day06")
	}
}

func TestHandleNavCreateKeyEscCancelsWithoutQuitting(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.navCreating = true
	m.navNewName.insert('x')
	m.navErr = "stale"

	next, cmd := m.handleNavCreateKey(tea.KeyMsg{Type: tea.KeyEsc})
	got := next.(debugModel)
	if got.navCreating {
		t.Error("expected navCreating to be cleared by Esc")
	}
	if got.navNewName.String() != "" {
		t.Errorf("navNewName = %q, want cleared", got.navNewName.String())
	}
	if got.navErr != "" {
		t.Errorf("navErr = %q, want cleared", got.navErr)
	}
	if cmd != nil {
		t.Error("expected Esc to cancel, not quit (nil Cmd)")
	}
}

func TestHandleNavCreateKeyCtrlCQuits(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.navCreating = true

	_, cmd := m.handleNavCreateKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected ctrl+c to return tea.Quit")
	}
	if msg := cmd(); msg != tea.Quit() {
		t.Errorf("cmd() = %v, want tea.Quit()", msg)
	}
}

func TestCreateNavFileEmptyNameIsError(t *testing.T) {
	dir := t.TempDir()
	day01 := writeCrustFileIn(t, dir, "day01.crust", "x = 1")
	m := newDebugModel(&debugView{path: day01, rec: viewFor(t, "x = 1").rec})
	m.navCreating = true

	next, _ := m.createNavFile()
	got := next.(debugModel)
	if got.navErr == "" {
		t.Error("expected navErr for an empty name")
	}
	if !got.navCreating {
		t.Error("expected navCreating to stay true so the prompt stays up")
	}
}

func TestCreateNavFileAppendsCrustSuffix(t *testing.T) {
	dir := t.TempDir()
	day01 := writeCrustFileIn(t, dir, "day01.crust", "x = 1")
	m := newDebugModel(&debugView{path: day01, rec: viewFor(t, "x = 1").rec})
	m.navCreating = true
	m.navNewName.insert('d')
	m.navNewName.insert('a')
	m.navNewName.insert('y')
	m.navNewName.insert('0')
	m.navNewName.insert('6')

	next, _ := m.createNavFile()
	got := next.(debugModel)
	if got.navErr != "" {
		t.Fatalf("createNavFile: %s", got.navErr)
	}
	wantPath := filepath.Join(dir, "day06.crust")
	if got.view.path != wantPath {
		t.Errorf("view.path = %q, want %q", got.view.path, wantPath)
	}
	if _, err := os.Stat(wantPath); err != nil {
		t.Errorf("file was not created: %s", err)
	}
	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Errorf("created file has %d bytes, want empty", len(data))
	}
}

func TestCreateNavFileKeepsExplicitCrustSuffix(t *testing.T) {
	dir := t.TempDir()
	day01 := writeCrustFileIn(t, dir, "day01.crust", "x = 1")
	m := newDebugModel(&debugView{path: day01, rec: viewFor(t, "x = 1").rec})
	m.navCreating = true
	for _, r := range "day06.crust" {
		m.navNewName.insert(r)
	}

	next, _ := m.createNavFile()
	got := next.(debugModel)
	wantPath := filepath.Join(dir, "day06.crust")
	if got.view.path != wantPath {
		t.Errorf("view.path = %q, want %q (no doubled suffix)", got.view.path, wantPath)
	}
}

func TestCreateNavFileSwitchesToTheNewFile(t *testing.T) {
	dir := t.TempDir()
	day01 := writeCrustFileIn(t, dir, "day01.crust", "x = 1")
	m := newDebugModel(&debugView{path: day01, rec: viewFor(t, "x = 1").rec})
	m.navCreating = true
	m.active = tabNav
	for _, r := range "day06" {
		m.navNewName.insert(r)
	}

	next, _ := m.createNavFile()
	got := next.(debugModel)
	if got.navCreating {
		t.Error("expected navCreating cleared after a successful create")
	}
	if got.active != tabTime {
		t.Errorf("active = %v, want tabTime after switching to the new file", got.active)
	}
	if got.view.rec.Steps() != 0 {
		t.Error("expected a fresh, unrecorded view of the brand-new file")
	}
}

func TestCreateNavFileAlreadyExistsIsError(t *testing.T) {
	dir := t.TempDir()
	day01 := writeCrustFileIn(t, dir, "day01.crust", "x = 1")
	writeCrustFileIn(t, dir, "day02.crust", `deliver("already here")`)
	m := newDebugModel(&debugView{path: day01, rec: viewFor(t, "x = 1").rec})
	m.navCreating = true
	for _, r := range "day02" {
		m.navNewName.insert(r)
	}

	next, _ := m.createNavFile()
	got := next.(debugModel)
	if got.navErr == "" {
		t.Error("expected navErr for a name that already exists")
	}
	if !got.navCreating {
		t.Error("expected navCreating to stay true so the prompt stays up")
	}
	data, err := os.ReadFile(filepath.Join(dir, "day02.crust"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `deliver("already here")` {
		t.Error("expected the existing file to be left untouched")
	}
}

func TestViewNavShowsCreatePromptWhenCreating(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.navCreating = true
	m.navNewName.insert('d')

	out := m.viewNav()
	if !strings.Contains(out, "new file name") {
		t.Errorf("viewNav() = %q, want the create-prompt heading", out)
	}
	if !strings.Contains(out, "d") {
		t.Errorf("viewNav() = %q, want the typed name shown", out)
	}
}

func TestViewNavCreatePromptTakesPriorityOverNoSiblingsPlaceholder(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.refreshNavFiles() // only one file -- would normally show the placeholder
	m.navCreating = true

	out := m.viewNav()
	if strings.Contains(out, "no other") {
		t.Errorf("viewNav() = %q, want the create prompt, not the no-other-files placeholder", out)
	}
}

func TestHelpTextOnFilesTabWhileCreating(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabNav
	m.navCreating = true
	help := m.helpText()
	if !strings.Contains(help, "create") || !strings.Contains(help, "cancel") {
		t.Errorf("helpText() = %q, want it to mention create/cancel", help)
	}
}

// --- create-new-AoC-day (ctrl+n) ----------------------------------------

func TestHandleKeyCtrlNStartsAoCCreating(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabNav

	next, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlN})
	got := next.(debugModel)
	if !got.navCreatingAoC {
		t.Error("expected navCreatingAoC to be true after pressing ctrl+n on the Files tab")
	}
	if got.navCreating {
		t.Error("expected plain navCreating to stay false when starting the AoC-day prompt")
	}
}

func TestHandleKeyCtrlNElsewhereIsNoOp(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabTime

	next, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlN})
	got := next.(debugModel)
	if got.navCreatingAoC {
		t.Error("expected ctrl+n to be a no-op off the Files tab")
	}
}

func TestHandleNavCreateKeyEnterRoutesToAoCWhenNavCreatingAoC(t *testing.T) {
	dir := t.TempDir()
	day01 := writeCrustFileIn(t, dir, "day01.crust", "x = 1")
	m := newDebugModel(&debugView{path: day01, rec: viewFor(t, "x = 1").rec})
	m.navCreatingAoC = true
	for _, r := range "7" {
		m.navNewName.insert(r)
	}

	next, _ := m.handleNavCreateKey(tea.KeyMsg{Type: tea.KeyEnter})
	got := next.(debugModel)
	wantPath := filepath.Join(dir, "day07.crust")
	if got.view.path != wantPath {
		t.Errorf("view.path = %q, want %q", got.view.path, wantPath)
	}
}

func TestHandleNavCreateKeyEscClearsAoCCreating(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.navCreatingAoC = true
	m.navNewName.insert('7')
	m.navErr = "stale"

	next, _ := m.handleNavCreateKey(tea.KeyMsg{Type: tea.KeyEsc})
	got := next.(debugModel)
	if got.navCreatingAoC {
		t.Error("expected navCreatingAoC to be cleared by Esc")
	}
	if got.navNewName.String() != "" {
		t.Errorf("navNewName = %q, want cleared", got.navNewName.String())
	}
}

func TestCreateNavAoCFileDayOnly(t *testing.T) {
	dir := t.TempDir()
	day01 := writeCrustFileIn(t, dir, "day01.crust", "x = 1")
	m := newDebugModel(&debugView{path: day01, rec: viewFor(t, "x = 1").rec})
	m.navCreatingAoC = true
	m.active = tabNav
	for _, r := range "9" {
		m.navNewName.insert(r)
	}

	next, _ := m.createNavAoCFile()
	got := next.(debugModel)
	if got.navErr != "" {
		t.Fatalf("createNavAoCFile: %s", got.navErr)
	}
	wantPath := filepath.Join(dir, "day09.crust")
	if got.view.path != wantPath {
		t.Errorf("view.path = %q, want %q", got.view.path, wantPath)
	}
	if got.navCreatingAoC {
		t.Error("expected navCreatingAoC cleared after a successful create")
	}
	if got.active != tabTime {
		t.Errorf("active = %v, want tabTime after switching to the new file", got.active)
	}
	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "store_part1") {
		t.Errorf("created file = %q, want the AoC starter template (store_part1)", data)
	}
}

func TestCreateNavAoCFileWithYear(t *testing.T) {
	withTempDevelStateDir(t)
	dir := t.TempDir()
	day01 := writeCrustFileIn(t, dir, "day01.crust", "x = 1")
	m := newDebugModel(&debugView{path: day01, rec: viewFor(t, "x = 1").rec})
	m.navCreatingAoC = true
	for _, r := range "9 2022" {
		m.navNewName.insert(r)
	}

	next, _ := m.createNavAoCFile()
	got := next.(debugModel)
	if got.navErr != "" {
		t.Fatalf("createNavAoCFile: %s", got.navErr)
	}
	wantPath := filepath.Join(dir, "day09.crust")
	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "2022") {
		t.Errorf("created file = %q, want the given year (2022) in the template", data)
	}

	all := loadDevelState()
	abs, _ := filepath.Abs(wantPath)
	if all[abs].Year != 2022 {
		t.Errorf("persisted Year = %d, want 2022", all[abs].Year)
	}
}

func TestCreateNavAoCFileInvalidDay(t *testing.T) {
	dir := t.TempDir()
	day01 := writeCrustFileIn(t, dir, "day01.crust", "x = 1")
	m := newDebugModel(&debugView{path: day01, rec: viewFor(t, "x = 1").rec})
	m.navCreatingAoC = true
	for _, r := range "banana" {
		m.navNewName.insert(r)
	}

	next, _ := m.createNavAoCFile()
	got := next.(debugModel)
	if got.navErr == "" {
		t.Error("expected navErr for a non-numeric day")
	}
	if !got.navCreatingAoC {
		t.Error("expected navCreatingAoC to stay true so the prompt stays up")
	}
}

func TestCreateNavAoCFileOutOfRangeDay(t *testing.T) {
	dir := t.TempDir()
	day01 := writeCrustFileIn(t, dir, "day01.crust", "x = 1")
	m := newDebugModel(&debugView{path: day01, rec: viewFor(t, "x = 1").rec})
	m.navCreatingAoC = true
	for _, r := range "26" {
		m.navNewName.insert(r)
	}

	next, _ := m.createNavAoCFile()
	got := next.(debugModel)
	if got.navErr == "" {
		t.Error("expected navErr for an out-of-range day (26)")
	}
}

func TestCreateNavAoCFileTooManyFields(t *testing.T) {
	dir := t.TempDir()
	day01 := writeCrustFileIn(t, dir, "day01.crust", "x = 1")
	m := newDebugModel(&debugView{path: day01, rec: viewFor(t, "x = 1").rec})
	m.navCreatingAoC = true
	for _, r := range "7 2020 extra" {
		m.navNewName.insert(r)
	}

	next, _ := m.createNavAoCFile()
	got := next.(debugModel)
	if got.navErr == "" {
		t.Error("expected navErr for more than a day + year")
	}
}

func TestCreateNavAoCFileEmptyIsError(t *testing.T) {
	dir := t.TempDir()
	day01 := writeCrustFileIn(t, dir, "day01.crust", "x = 1")
	m := newDebugModel(&debugView{path: day01, rec: viewFor(t, "x = 1").rec})
	m.navCreatingAoC = true

	next, _ := m.createNavAoCFile()
	got := next.(debugModel)
	if got.navErr == "" {
		t.Error("expected navErr for an empty day field")
	}
}

func TestCreateNavAoCFileAlreadyExistsIsError(t *testing.T) {
	dir := t.TempDir()
	day01 := writeCrustFileIn(t, dir, "day01.crust", "x = 1")
	writeCrustFileIn(t, dir, "day09.crust", `deliver("already here")`)
	m := newDebugModel(&debugView{path: day01, rec: viewFor(t, "x = 1").rec})
	m.navCreatingAoC = true
	for _, r := range "9" {
		m.navNewName.insert(r)
	}

	next, _ := m.createNavAoCFile()
	got := next.(debugModel)
	if got.navErr == "" {
		t.Error("expected navErr for a day file that already exists")
	}
	data, err := os.ReadFile(filepath.Join(dir, "day09.crust"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `deliver("already here")` {
		t.Error("expected the existing file to be left untouched")
	}
}

func TestViewNavShowsAoCPromptHeading(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.navCreatingAoC = true
	m.navNewName.insert('7')

	out := m.viewNav()
	if !strings.Contains(out, "new AoC day") {
		t.Errorf("viewNav() = %q, want the AoC-day create-prompt heading", out)
	}
	if !strings.Contains(out, "7") {
		t.Errorf("viewNav() = %q, want the typed day shown", out)
	}
}

func TestHelpTextOnFilesTabMentionsCtrlN(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabNav
	if !strings.Contains(m.helpText(), "ctrl+n") {
		t.Errorf("helpText() = %q, want it to mention ctrl+n", m.helpText())
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
