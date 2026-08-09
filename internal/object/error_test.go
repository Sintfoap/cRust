package object

import (
	"strings"
	"testing"
)

func TestError(t *testing.T) {
	err := &Error{Message: "division by zero", Line: 4, Col: 9}

	if err.Type() != ERROR_OBJ {
		t.Errorf("Type() = %v, want %v", err.Type(), ERROR_OBJ)
	}
	if want := "Error: division by zero"; err.Inspect() != want {
		t.Errorf("Inspect() = %q, want %q", err.Inspect(), want)
	}
}

func TestFrameLinesNilWithNoFrames(t *testing.T) {
	err := &Error{Message: "boom"}
	if lines := err.FrameLines(); lines != nil {
		t.Errorf("FrameLines() = %v, want nil for no frames", lines)
	}
}

func TestFrameLinesNilWithOneFrame(t *testing.T) {
	err := &Error{Message: "boom", Frames: []Frame{{Name: "store(...)", Line: 0, Col: 0}}}
	if lines := err.FrameLines(); lines != nil {
		t.Errorf("FrameLines() = %v, want nil for a single frame (adds nothing beyond the primary line)", lines)
	}
}

func TestFrameLinesTwoLevels(t *testing.T) {
	err := &Error{Message: "boom", Frames: []Frame{
		{Name: "divide(...)", Line: 9, Col: 12},
		{Name: "store(...)", Line: 0, Col: 0},
	}}
	got := err.FrameLines()
	want := []string{
		"    in divide(...), called from 9:12",
		"    in store(...)",
	}
	if len(got) != len(want) {
		t.Fatalf("FrameLines() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("FrameLines()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFrameLinesCapsDeepChains(t *testing.T) {
	frames := make([]Frame, 30)
	for i := range frames {
		frames[i] = Frame{Name: "boom(...)", Line: 5, Col: 15}
	}
	err := &Error{Message: "boom", Frames: frames}
	got := err.FrameLines()

	if len(got) != maxErrorFrames+1 {
		t.Fatalf("FrameLines() has %d lines, want %d (%d shown + 1 omitted-count line)", len(got), maxErrorFrames+1, maxErrorFrames)
	}
	last := got[len(got)-1]
	if !strings.Contains(last, "18 more") {
		t.Errorf("last line = %q, want it to mention the 18 omitted frames (30 - %d)", last, maxErrorFrames)
	}
}
