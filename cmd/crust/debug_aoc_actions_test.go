package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Sintfoap/cRust/internal/aoc"
)

func TestLevelForEntry(t *testing.T) {
	tests := []struct {
		store string
		want  int
	}{
		{"", 1},
		{"part1", 1},
		{"part2", 2},
		{"something-else", 1},
	}
	for _, tt := range tests {
		if got := levelForEntry(tt.store); got != tt.want {
			t.Errorf("levelForEntry(%q) = %d, want %d", tt.store, got, tt.want)
		}
	}
}

func TestLastNonEmptyLine(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"42", "42"},
		{"line1\nline2\n42\n", "42"},
		{"42\n\n\n", "42"},
		{"  42  \n", "42"},
		{"\n\n", ""},
	}
	for _, tt := range tests {
		if got := lastNonEmptyLine(tt.in); got != tt.want {
			t.Errorf("lastNonEmptyLine(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// --- ctrl+f fetch ---------------------------------------------------------

func TestAocFetchCmdNonDayFileIsError(t *testing.T) {
	withTempConfigHome(t)
	path := writeAocDebugFile(t, t.TempDir(), "scratch.crust")
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})

	msg := m.aocFetchCmd()().(aocFetchMsg)
	if msg.err == "" {
		t.Fatal("expected an error for a non-dayNN.crust file")
	}
}

func TestAocFetchCmdNoSessionIsError(t *testing.T) {
	withTempConfigHome(t)
	path := writeAocDebugFile(t, t.TempDir(), "day03.crust")
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.opts.Year = 2026

	msg := m.aocFetchCmd()().(aocFetchMsg)
	if !strings.Contains(msg.err, "ctrl+l") {
		t.Errorf("err = %q, want it to point at ctrl+l", msg.err)
	}
}

func TestAocFetchCmdDownloadsAndWiresInputField(t *testing.T) {
	withTempConfigHome(t)
	if err := aoc.SaveSession("s"); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("puzzle input\n"))
	}))
	defer srv.Close()
	old := fetchBaseURL
	fetchBaseURL = srv.URL
	t.Cleanup(func() { fetchBaseURL = old })

	dir := t.TempDir()
	path := writeAocDebugFile(t, dir, "day03.crust")
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.opts.Year = 2026

	msg := m.aocFetchCmd()().(aocFetchMsg)
	if msg.err != "" {
		t.Fatalf("aocFetchCmd: %s", msg.err)
	}
	wantPath := aocInputPath(path, 3)
	if msg.inputPath != wantPath {
		t.Errorf("inputPath = %q, want %q", msg.inputPath, wantPath)
	}
	data, err := os.ReadFile(wantPath)
	if err != nil || string(data) != "puzzle input\n" {
		t.Errorf("input file = %q, %v", data, err)
	}
	if _, running := aoc.Elapsed(2026, 3); !running {
		t.Error("expected the timer to start")
	}

	got, _ := m.handleAocFetchResult(msg)
	gotM := got.(debugModel)
	if gotM.runInput.String() != wantPath {
		t.Errorf("runInput = %q, want %q", gotM.runInput.String(), wantPath)
	}
	if gotM.aocActionStatus == "" {
		t.Error("expected a non-empty status message")
	}

	saved := loadDevelState()[mustAbs(t, path)]
	if saved.Input != wantPath {
		t.Errorf("persisted Input = %q, want %q", saved.Input, wantPath)
	}
}

func TestAocFetchCmdAlreadyPresentWiresPathWithoutRefetch(t *testing.T) {
	withTempConfigHome(t)
	dir := t.TempDir()
	path := writeAocDebugFile(t, dir, "day03.crust")
	inputPath := aocInputPath(path, 3)
	if err := os.WriteFile(inputPath, []byte("existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.opts.Year = 2026

	msg := m.aocFetchCmd()().(aocFetchMsg)
	if msg.err != "" {
		t.Fatalf("aocFetchCmd: %s", msg.err)
	}
	if msg.inputPath != inputPath {
		t.Errorf("inputPath = %q, want %q", msg.inputPath, inputPath)
	}
	data, _ := os.ReadFile(inputPath)
	if string(data) != "existing\n" {
		t.Errorf("existing input file was overwritten: %q", data)
	}
}

func TestHandleRunTabKeyCtrlFDispatchesFetchCmd(t *testing.T) {
	withTempConfigHome(t)
	path := writeAocDebugFile(t, t.TempDir(), "day03.crust")
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.active = tabRun

	_, cmd := m.handleRunTabKey(tea.KeyMsg{Type: tea.KeyCtrlF})
	if cmd == nil {
		t.Fatal("expected ctrl+f to return a command")
	}
	if _, ok := cmd().(aocFetchMsg); !ok {
		t.Errorf("cmd() = %T, want aocFetchMsg", cmd())
	}
}

// --- ctrl+s submit ----------------------------------------------------------

func TestAocSubmitCmdNoRunYetIsError(t *testing.T) {
	withTempConfigHome(t)
	path := writeAocDebugFile(t, t.TempDir(), "day03.crust")
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.opts.Year = 2026

	msg := m.aocSubmitCmd()().(aocSubmitMsg)
	if msg.err == "" {
		t.Fatal("expected an error when nothing has been run yet")
	}
}

func TestAocSubmitCmdFailedRunIsError(t *testing.T) {
	withTempConfigHome(t)
	path := writeAocDebugFile(t, t.TempDir(), "day03.crust")
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.opts.Year = 2026
	m.runOutput = "some error trace"
	m.runFailed = true

	msg := m.aocSubmitCmd()().(aocSubmitMsg)
	if !strings.Contains(msg.err, "failed") {
		t.Errorf("err = %q, want it to mention the failed run", msg.err)
	}
}

func TestAocSubmitCmdNoSessionIsError(t *testing.T) {
	withTempConfigHome(t)
	path := writeAocDebugFile(t, t.TempDir(), "day03.crust")
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.opts.Year = 2026
	m.runOutput = "42"

	msg := m.aocSubmitCmd()().(aocSubmitMsg)
	if !strings.Contains(msg.err, "ctrl+l") {
		t.Errorf("err = %q, want it to point at ctrl+l", msg.err)
	}
}

func TestAocSubmitCmdSubmitsLastLineAndReportsVerdict(t *testing.T) {
	withTempConfigHome(t)
	if err := aoc.SaveSession("s"); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	var gotAnswer, gotLevel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		gotAnswer = r.FormValue("answer")
		gotLevel = r.FormValue("level")
		w.Write([]byte(`<article><p>That's the right answer!</p></article>`))
	}))
	defer srv.Close()
	old := submitBaseURL
	submitBaseURL = srv.URL
	t.Cleanup(func() { submitBaseURL = old })

	path := writeAocDebugFile(t, t.TempDir(), "day03.crust")
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.opts.Year = 2026
	m.runOutput = "some intro line\n42\n"

	msg := m.aocSubmitCmd()().(aocSubmitMsg)
	if msg.err != "" {
		t.Fatalf("aocSubmitCmd: %s", msg.err)
	}
	if gotAnswer != "42" {
		t.Errorf("submitted answer = %q, want %q (the last non-empty line)", gotAnswer, "42")
	}
	if gotLevel != "1" {
		t.Errorf("submitted level = %q, want 1 (default/bare store)", gotLevel)
	}
	if !strings.Contains(msg.status, "Correct") {
		t.Errorf("status = %q, want it to say Correct", msg.status)
	}

	got, _ := m.handleAocSubmitResult(msg)
	if got.(debugModel).aocActionStatus != msg.status {
		t.Error("expected handleAocSubmitResult to set aocActionStatus")
	}
}

func TestAocSubmitCmdUsesLevelFromSelectedEntry(t *testing.T) {
	withTempConfigHome(t)
	if err := aoc.SaveSession("s"); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	var gotLevel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		gotLevel = r.FormValue("level")
		w.Write([]byte(`<article><p>That's not the right answer.</p></article>`))
	}))
	defer srv.Close()
	old := submitBaseURL
	submitBaseURL = srv.URL
	t.Cleanup(func() { submitBaseURL = old })

	dir := t.TempDir()
	path := writeCrustFileIn(t, dir, "day03.crust", `
recipe store_part1() { deliver("one") }
recipe store_part2() { deliver("two") }`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.opts.Year = 2026
	m.runEntryIndex = indexOfEntry(m.runEntryOptions(), "part2")
	m.runOutput = "99"

	msg := m.aocSubmitCmd()().(aocSubmitMsg)
	if msg.err != "" {
		t.Fatalf("aocSubmitCmd: %s", msg.err)
	}
	if gotLevel != "2" {
		t.Errorf("submitted level = %q, want 2 (part2 selected)", gotLevel)
	}
}

func TestHandleRunTabKeyCtrlSDispatchesSubmitCmd(t *testing.T) {
	withTempConfigHome(t)
	path := writeAocDebugFile(t, t.TempDir(), "day03.crust")
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.active = tabRun

	_, cmd := m.handleRunTabKey(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd == nil {
		t.Fatal("expected ctrl+s to return a command")
	}
	if _, ok := cmd().(aocSubmitMsg); !ok {
		t.Errorf("cmd() = %T, want aocSubmitMsg", cmd())
	}
}

// --- ctrl+l login -----------------------------------------------------------

func TestHandleLoginExitSessionSaved(t *testing.T) {
	withTempConfigHome(t)
	if err := aoc.SaveSession("s"); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	m := newDebugModel(viewFor(t, "x = 1"))

	got, _ := m.handleLoginExit(loginExitMsg{})
	if !strings.Contains(got.(debugModel).aocActionStatus, "saved") {
		t.Errorf("aocActionStatus = %q, want it to mention the session was saved", got.(debugModel).aocActionStatus)
	}
}

func TestHandleLoginExitNoSessionSaved(t *testing.T) {
	withTempConfigHome(t)
	m := newDebugModel(viewFor(t, "x = 1"))

	got, _ := m.handleLoginExit(loginExitMsg{})
	if !strings.Contains(got.(debugModel).aocActionStatus, "without saving") {
		t.Errorf("aocActionStatus = %q, want it to mention no session was saved", got.(debugModel).aocActionStatus)
	}
}

func TestHandleRunTabKeyCtrlLDispatchesLoginCmd(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabRun

	_, cmd := m.handleRunTabKey(tea.KeyMsg{Type: tea.KeyCtrlL})
	if cmd == nil {
		t.Fatal("expected ctrl+l to return a command")
	}
}

// --- view / layout ----------------------------------------------------------

func TestViewRunShowsAocActionStatus(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.width, m.height = 100, 40
	m.aocActionStatus = "fetched day 3, 2026 input (12 bytes) — timer started"

	out := m.viewRun()
	if !strings.Contains(out, "fetched day 3, 2026") {
		t.Errorf("viewRun() = %q, want the status shown", out)
	}
}

func TestRunOutputExtraLinesAccountsForAocStatus(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	base := m.runOutputExtraLines()
	m.aocActionStatus = "something happened"
	if got := m.runOutputExtraLines(); got != base+2 {
		t.Errorf("runOutputExtraLines() = %d, want %d (base + 2)", got, base+2)
	}
}
