package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Sintfoap/cRust/internal/debugger"
	"github.com/Sintfoap/cRust/internal/object"
)

func TestReadLiveSourceReadsLines(t *testing.T) {
	path := writeDebugFile(t, "x = 1\ny = 2\nz = 3\n")
	got := readLiveSource(path)
	want := []string{"x = 1", "y = 2", "z = 3"}
	if len(got) != len(want) {
		t.Fatalf("readLiveSource() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestReadLiveSourceMissingFileReturnsNil(t *testing.T) {
	if got := readLiveSource("/no/such/file.crust"); got != nil {
		t.Errorf("readLiveSource() = %v, want nil", got)
	}
}

func TestReadLiveSourceEmptyFileReturnsNil(t *testing.T) {
	path := writeDebugFile(t, "")
	if got := readLiveSource(path); got != nil {
		t.Errorf("readLiveSource() = %v, want nil for an empty file", got)
	}
}

func waitLiveMsg(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	msgCh := make(chan tea.Msg, 1)
	go func() { msgCh <- cmd() }()
	select {
	case msg := <-msgCh:
		return msg
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for a live Cmd's message")
		return nil
	}
}

func TestStartLiveRunParseErrorReturnsError(t *testing.T) {
	path := writeDebugFile(t, "order (a < b {\n serve 1\n}\n")
	_, _, _, err := startLiveRun(path, debugOptions{}, strings.NewReader(""), nil)
	if err == nil {
		t.Fatal("startLiveRun() error = nil, want a parse error")
	}
}

func TestStartLiveRunPausesOnFirstStatement(t *testing.T) {
	path := writeDebugFile(t, "x = 1\ny = 2\n")
	lt, out, done, err := startLiveRun(path, debugOptions{}, strings.NewReader(""), nil)
	if err != nil {
		t.Fatalf("startLiveRun() error = %v", err)
	}
	select {
	case p := <-lt.Paused:
		if p.Line != 1 {
			t.Errorf("Line = %d, want 1", p.Line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the first pause")
	}
	lt.Resume <- debugger.LiveStop
	select {
	case res := <-done:
		if !res.Stopped {
			t.Errorf("done = %+v, want Stopped", res)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the run to finish")
	}
	_ = out
}

func TestStartLiveCmdReturnsImmediatePauseMsg(t *testing.T) {
	path := writeDebugFile(t, `recipe store() { deliver("hi") }`)
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})

	msg := waitLiveMsg(t, m.startLiveCmd(7))
	sm, ok := msg.(liveStartMsg)
	if !ok {
		t.Fatalf("msg = %T, want liveStartMsg", msg)
	}
	if sm.gen != 7 {
		t.Errorf("gen = %d, want 7", sm.gen)
	}
	if sm.err != nil {
		t.Fatalf("err = %v, want nil", sm.err)
	}
	if sm.pause == nil {
		t.Fatal("pause = nil, want an immediate pause (LiveTracer starts in step mode)")
	}
	if sm.pause.Line != 1 {
		t.Errorf("pause.Line = %d, want 1", sm.pause.Line)
	}

	sm.tracer.Resume <- debugger.LiveStop
}

func TestStartLiveCmdMissingInputFileReturnsError(t *testing.T) {
	path := writeDebugFile(t, "x = 1\n")
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.runInput = runInputModel{value: []rune("/does/not/exist.txt")}

	msg := waitLiveMsg(t, m.startLiveCmd(1))
	sm, ok := msg.(liveStartMsg)
	if !ok {
		t.Fatalf("msg = %T, want liveStartMsg", msg)
	}
	if sm.err == nil {
		t.Fatal("err = nil, want an error for a missing input file")
	}
}

func TestLiveResumeCmdStepAdvancesToNextPause(t *testing.T) {
	path := writeDebugFile(t, "x = 1\ny = 2\n")
	lt, out, done, err := startLiveRun(path, debugOptions{}, strings.NewReader(""), nil)
	if err != nil {
		t.Fatalf("startLiveRun() error = %v", err)
	}
	<-lt.Paused // line 1

	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.liveTracer, m.liveDone, m.liveGen = lt, done, 3

	msg := waitLiveMsg(t, m.liveResumeCmd(debugger.LiveStep))
	pm, ok := msg.(livePauseMsg)
	if !ok {
		t.Fatalf("msg = %T, want livePauseMsg", msg)
	}
	if pm.gen != 3 {
		t.Errorf("gen = %d, want 3", pm.gen)
	}
	if pm.Line != 2 {
		t.Errorf("Line = %d, want 2", pm.Line)
	}

	lt.Resume <- debugger.LiveStop
	_ = out
}

func TestHandleLiveStartIgnoresStaleGen(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.liveGen = 5
	next, cmd := m.handleLiveStart(liveStartMsg{gen: 4, err: nil})
	if cmd != nil {
		t.Errorf("cmd = %v, want nil for a stale message", cmd)
	}
	nm := next.(debugModel)
	if nm.liveRunning {
		t.Errorf("liveRunning = true, want the stale message to be ignored entirely")
	}
}

func TestHandleLiveStartAppliesErrorAndDoesNotMarkRunning(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	next, _ := m.handleLiveStart(liveStartMsg{gen: 0, err: errParseStub{}})
	nm := next.(debugModel)
	if nm.liveRunning {
		t.Error("liveRunning = true, want false when starting failed")
	}
	if nm.liveStatus == "" {
		t.Error("liveStatus is empty, want the start error's message")
	}
}

// errParseStub is a minimal error for TestHandleLiveStartAppliesErrorAndDoesNotMarkRunning.
type errParseStub struct{}

func (errParseStub) Error() string { return "boom" }

func TestHandleLivePauseIgnoresStaleGen(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.liveGen = 2
	next, _ := m.handleLivePause(livePauseMsg{gen: 1, LivePause: debugger.LivePause{Line: 5}})
	nm := next.(debugModel)
	if nm.livePaused {
		t.Error("livePaused = true, want the stale pause message to be ignored")
	}
}

func TestHandleLiveDoneStates(t *testing.T) {
	tests := []struct {
		name string
		msg  liveDoneMsg
		want string
	}{
		{"stopped", liveDoneMsg{Stopped: true}, "stopped"},
		{"failed", liveDoneMsg{Failed: true, Message: "1:1: division by zero"}, "1:1: division by zero"},
		{"finished", liveDoneMsg{}, "finished"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newDebugModel(viewFor(t, "x = 1"))
			m.liveRunning, m.livePaused, m.livePausedAt = true, true, 3
			next, _ := m.handleLiveDone(tt.msg)
			nm := next.(debugModel)
			if nm.liveRunning || nm.livePaused || nm.livePausedAt != 0 {
				t.Errorf("run-state fields not cleared: running=%v paused=%v pausedAt=%d", nm.liveRunning, nm.livePaused, nm.livePausedAt)
			}
			if nm.liveStatus != tt.want {
				t.Errorf("liveStatus = %q, want %q", nm.liveStatus, tt.want)
			}
		})
	}
}

func TestApplyLivePauseUpdatesCursorAndEnv(t *testing.T) {
	m := newDebugModel(&debugView{path: writeDebugFile(t, "x = 1\ny = 2\nz = 3\n"), rec: viewFor(t, "x = 1").rec})
	m.liveSource = readLiveSource(m.view.path)
	m.applyLivePause(debugger.LivePause{Line: 2, Label: "y = 2", Env: map[string]object.Object{}})
	if !m.livePaused {
		t.Error("livePaused = false, want true")
	}
	if m.livePausedAt != 2 {
		t.Errorf("livePausedAt = %d, want 2", m.livePausedAt)
	}
	if m.liveCursor != 1 {
		t.Errorf("liveCursor = %d, want 1 (0-based for line 2)", m.liveCursor)
	}
}

func TestToggleLiveBreakpointTogglesAndPersists(t *testing.T) {
	withTempDevelStateDir(t)
	path := writeDebugFile(t, "x = 1\ny = 2\n")
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.liveSource = readLiveSource(path)
	m.liveBreakpoints = map[int]bool{}
	m.liveCursor = 1 // line 2

	m.toggleLiveBreakpoint()
	if !m.liveBreakpoints[2] {
		t.Fatal("breakpoint on line 2 was not set")
	}
	if got := restoreLiveBreakpoints(path); !got[2] {
		t.Errorf("restoreLiveBreakpoints() = %v, want line 2 persisted", got)
	}

	m.toggleLiveBreakpoint()
	if m.liveBreakpoints[2] {
		t.Fatal("breakpoint on line 2 was not cleared by toggling again")
	}
	if got := restoreLiveBreakpoints(path); got[2] {
		t.Errorf("restoreLiveBreakpoints() = %v, want line 2 no longer persisted", got)
	}
}

func TestToggleLiveBreakpointPushesToRunningTracer(t *testing.T) {
	path := writeDebugFile(t, "x = 1\ny = 2\nz = 3\n")
	lt, _, done, err := startLiveRun(path, debugOptions{}, strings.NewReader(""), nil)
	if err != nil {
		t.Fatalf("startLiveRun() error = %v", err)
	}
	<-lt.Paused // line 1

	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.liveSource = readLiveSource(path)
	m.liveBreakpoints = map[int]bool{}
	m.liveTracer = lt
	m.liveCursor = 2 // line 3
	m.toggleLiveBreakpoint()

	lt.Resume <- debugger.LiveContinue
	select {
	case p := <-lt.Paused:
		if p.Line != 3 {
			t.Errorf("Line = %d, want 3 (the breakpoint just toggled onto the live tracer)", p.Line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the pushed breakpoint to fire")
	}
	lt.Resume <- debugger.LiveStop
	<-done
}

func TestMoveLiveCursorClamps(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.liveSource = []string{"a", "b", "c"}

	m.moveLiveCursor(-5)
	if m.liveCursor != 0 {
		t.Errorf("liveCursor = %d, want clamped to 0", m.liveCursor)
	}
	m.moveLiveCursor(10)
	if m.liveCursor != 2 {
		t.Errorf("liveCursor = %d, want clamped to 2 (len-1)", m.liveCursor)
	}
}

func TestMoveLiveCursorNoopWhenNoSource(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.liveCursor = 0
	m.moveLiveCursor(1)
	if m.liveCursor != 0 {
		t.Errorf("liveCursor = %d, want unchanged with no source loaded", m.liveCursor)
	}
}

func TestMaybeRefreshLiveOnlyOnLiveTab(t *testing.T) {
	path := writeDebugFile(t, "x = 1\ny = 2\n")
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.active = tabTime
	m.maybeRefreshLive()
	if m.liveSource != nil {
		t.Error("maybeRefreshLive() populated liveSource while not on the Live tab")
	}
	m.active = tabLive
	m.maybeRefreshLive()
	if len(m.liveSource) != 2 {
		t.Errorf("liveSource = %v, want 2 lines after switching to the Live tab", m.liveSource)
	}
}

func TestViewLiveShowsBreakpointAndCursorMarkers(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.liveSource = []string{"x = 1", "y = 2"}
	m.liveBreakpoints = map[int]bool{2: true}
	m.liveCursor = 1
	m.width, m.height = 80, 30

	out := m.viewLive()
	if !strings.Contains(out, "●") {
		t.Errorf("viewLive() = %q, want a breakpoint marker", out)
	}
	if !strings.Contains(out, "y = 2") {
		t.Errorf("viewLive() = %q, want the source line text", out)
	}
	if !strings.Contains(out, "not started") {
		t.Errorf("viewLive() = %q, want the not-started status", out)
	}
}

func TestViewLiveShowsPausedLineAndWatchPanel(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.liveSource = []string{"x = 1", "y = 2"}
	m.liveRunning = true
	m.width, m.height = 80, 30
	m.applyLivePause(debugger.LivePause{Line: 2, Label: "y = 2", Env: map[string]object.Object{}})

	out := m.viewLive()
	if !strings.Contains(out, "paused at line 2") {
		t.Errorf("viewLive() = %q, want the paused status line", out)
	}
}

// manyVarsEnv builds an env with more names than maxLiveWatchLines,
// numbered so their sorted order is predictable ("v00".."v09").
func manyVarsEnv(n int) map[string]object.Object {
	env := make(map[string]object.Object, n)
	for i := 0; i < n; i++ {
		env[fmt.Sprintf("v%02d", i)] = object.NewInteger(int64(i))
	}
	return env
}

func TestViewLiveWatchShowsAllWhenFewerThanMax(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.liveEnv = manyVarsEnv(3)
	out := m.viewLiveWatch()
	if !strings.Contains(out, "v00") || !strings.Contains(out, "v02") {
		t.Errorf("viewLiveWatch() = %q, want every variable shown", out)
	}
	if strings.Contains(out, "more above") || strings.Contains(out, "more below") {
		t.Errorf("viewLiveWatch() = %q, want no scroll hints when everything already fits", out)
	}
}

func TestViewLiveWatchScrollsWithMoreBelow(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.liveEnv = manyVarsEnv(10) // > maxLiveWatchLines (6)
	out := m.viewLiveWatch()
	if !strings.Contains(out, "v00") {
		t.Errorf("viewLiveWatch() = %q, want the first name visible at top=0", out)
	}
	if strings.Contains(out, "v09") {
		t.Errorf("viewLiveWatch() = %q, want the last name NOT visible yet", out)
	}
	if !strings.Contains(out, "more below") {
		t.Errorf("viewLiveWatch() = %q, want a \"more below\" hint", out)
	}
	if strings.Contains(out, "more above") {
		t.Errorf("viewLiveWatch() = %q, want no \"more above\" hint at the very top", out)
	}
}

func TestMoveLiveWatchCursorScrollsIntoView(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.liveEnv = manyVarsEnv(10)

	m.moveLiveWatchCursor(2)
	if m.liveWatchTop != 2 {
		t.Fatalf("liveWatchTop = %d, want 2", m.liveWatchTop)
	}
	out := m.viewLiveWatch()
	if strings.Contains(out, "v00") || strings.Contains(out, "v01") {
		t.Errorf("viewLiveWatch() = %q, want v00/v01 scrolled out of view", out)
	}
	if !strings.Contains(out, "v02") {
		t.Errorf("viewLiveWatch() = %q, want v02 now visible", out)
	}
	if !strings.Contains(out, "more above") || !strings.Contains(out, "more below") {
		t.Errorf("viewLiveWatch() = %q, want both scroll hints while scrolled to the middle", out)
	}
}

func TestMoveLiveWatchCursorClampsAtTopAndBottom(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.liveEnv = manyVarsEnv(10)

	m.moveLiveWatchCursor(-5)
	if m.liveWatchTop != 0 {
		t.Errorf("liveWatchTop = %d, want 0 (clamped, can't scroll above the top)", m.liveWatchTop)
	}

	m.moveLiveWatchCursor(100)
	wantMaxTop := 10 - maxLiveWatchLines
	if m.liveWatchTop != wantMaxTop {
		t.Errorf("liveWatchTop = %d, want %d (clamped at the bottom)", m.liveWatchTop, wantMaxTop)
	}
	out := m.viewLiveWatch()
	if !strings.Contains(out, "v09") {
		t.Errorf("viewLiveWatch() = %q, want the last name visible once scrolled to the bottom", out)
	}
	if strings.Contains(out, "more below") {
		t.Errorf("viewLiveWatch() = %q, want no \"more below\" hint at the very bottom", out)
	}
}

func TestMoveLiveWatchCursorNoopWhenFewerThanMax(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.liveEnv = manyVarsEnv(3)
	m.moveLiveWatchCursor(5)
	if m.liveWatchTop != 0 {
		t.Errorf("liveWatchTop = %d, want 0 (nothing to scroll to)", m.liveWatchTop)
	}
}

func TestHandleKeyLiveTabTogglesWatchFocusOnlyWhilePaused(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabLive

	// Not paused yet: 'w' does nothing.
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	dm := m2.(debugModel)
	if dm.liveWatchFocus {
		t.Error("liveWatchFocus = true, want false — 'w' shouldn't do anything before a pause")
	}

	dm.livePaused = true
	m3, _ := dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	dm = m3.(debugModel)
	if !dm.liveWatchFocus {
		t.Error("liveWatchFocus = false, want true after 'w' while paused")
	}

	m4, _ := dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	dm = m4.(debugModel)
	if dm.liveWatchFocus {
		t.Error("liveWatchFocus = true, want false — a second 'w' should toggle it back off")
	}
}

func TestHandleKeyLiveTabWatchFocusRedirectsJK(t *testing.T) {
	m := newDebugModel(viewFor(t, "x = 1"))
	m.active = tabLive
	m.livePaused = true
	m.liveWatchFocus = true
	m.liveEnv = manyVarsEnv(10)
	m.liveSource = []string{"a", "b", "c"}
	startCursor := m.liveCursor

	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	dm := m2.(debugModel)
	if dm.liveWatchTop != 1 {
		t.Errorf("liveWatchTop = %d, want 1 — j should scroll the watch panel while focused", dm.liveWatchTop)
	}
	if dm.liveCursor != startCursor {
		t.Errorf("liveCursor = %d, want unchanged (%d) — j shouldn't move the source cursor while watch-focused", dm.liveCursor, startCursor)
	}
}

func TestTailLines(t *testing.T) {
	got := tailLines("a\nb\nc\nd\n", 2)
	if !strings.Contains(got, "c") || !strings.Contains(got, "d") || strings.Contains(got, "\na\n") {
		t.Errorf("tailLines() = %q, want only the last 2 lines", got)
	}
	if got := tailLines("a\nb\n", 5); got != "a\nb" {
		t.Errorf("tailLines() = %q, want the whole (short) input unchanged", got)
	}
}

func TestHandleKeyLiveTabFullCycle(t *testing.T) {
	path := writeDebugFile(t, "x = 1\ny = 2\n")
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.active = tabLive
	m.liveSource = readLiveSource(path)
	m.liveBreakpoints = map[int]bool{}

	// 'r' starts a run: expect a Cmd, and running the returned Cmd
	// synchronously (the way waitLiveMsg already does elsewhere in this
	// file) should report an immediate pause on line 1.
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = next.(debugModel)
	if m.liveStatus != "starting…" {
		t.Errorf("liveStatus after 'r' = %q, want the immediate starting feedback", m.liveStatus)
	}
	if cmd == nil {
		t.Fatal("'r' returned a nil Cmd, want startLiveCmd")
	}
	msg := waitLiveMsg(t, cmd)
	next, _ = m.Update(msg)
	m = next.(debugModel)
	if !m.liveRunning || !m.livePaused || m.livePausedAt != 1 {
		t.Fatalf("after starting: running=%v paused=%v pausedAt=%d, want running+paused at line 1", m.liveRunning, m.livePaused, m.livePausedAt)
	}

	// 's' steps to line 2.
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = next.(debugModel)
	if m.livePaused {
		t.Error("livePaused should flip false immediately on 's', before the round trip completes")
	}
	msg = waitLiveMsg(t, cmd)
	next, _ = m.Update(msg)
	m = next.(debugModel)
	if !m.livePaused || m.livePausedAt != 2 {
		t.Fatalf("after step: paused=%v pausedAt=%d, want paused at line 2", m.livePaused, m.livePausedAt)
	}

	// 'x' while paused stops the run.
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = next.(debugModel)
	if cmd == nil {
		t.Fatal("'x' while paused returned a nil Cmd, want liveResumeCmd(LiveStop)")
	}
	msg = waitLiveMsg(t, cmd)
	next, _ = m.Update(msg)
	m = next.(debugModel)
	if m.liveRunning {
		t.Error("liveRunning = true after stopping, want false")
	}
	if m.liveStatus != "stopped" {
		t.Errorf("liveStatus = %q, want %q", m.liveStatus, "stopped")
	}
}

func TestHandleKeyLiveTabBreakpointToggle(t *testing.T) {
	withTempDevelStateDir(t)
	path := writeDebugFile(t, "x = 1\ny = 2\n")
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.active = tabLive
	m.liveSource = readLiveSource(path)
	m.liveBreakpoints = map[int]bool{}
	m.liveCursor = 1 // line 2

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	m = next.(debugModel)
	if !m.liveBreakpoints[2] {
		t.Error("'b' did not set a breakpoint on the cursor's line")
	}
}

func TestHandleKeyLiveTabMoveCursor(t *testing.T) {
	path := writeDebugFile(t, "x = 1\ny = 2\nz = 3\n")
	m := newDebugModel(&debugView{path: path, rec: viewFor(t, "x = 1").rec})
	m.active = tabLive
	m.liveSource = readLiveSource(path)
	m.width, m.height = 80, 30

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = next.(debugModel)
	if m.liveCursor != 1 {
		t.Errorf("liveCursor after 'j' = %d, want 1", m.liveCursor)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = next.(debugModel)
	if m.liveCursor != 0 {
		t.Errorf("liveCursor after 'k' = %d, want 0", m.liveCursor)
	}
}
