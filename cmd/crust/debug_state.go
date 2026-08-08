// Per-file `crust develop` settings, remembered across invocations:
// which store/store_<name> entry point and which Run-tab input-file
// path were last used for a given .crust file, so reopening it later
// starts back where you left off instead of always defaulting to the
// bare store and an empty input field.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// develState is one file's remembered settings.
type develState struct {
	Store      string `json:"store"`
	Input      string `json:"input"`
	RunAll     bool   `json:"runAll,omitempty"`
	BenchCount int    `json:"benchCount,omitempty"`
}

// develStateDir resolves to os.UserConfigDir() normally; tests
// override it to a temp directory so they never read or write the
// real user's config directory.
var develStateDir = os.UserConfigDir

// develStatePath is the single JSON file every debugged file's
// settings live in, keyed by absolute path — one shared file rather
// than one dotfile dropped next to every .crust file, which would
// litter every project directory with something crust itself would
// then have to know to ignore.
func develStatePath() (string, error) {
	dir, err := develStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "crust", "develop_state.json"), nil
}

// loadDevelState reads every remembered file's settings. Any failure
// to read or parse — the file doesn't exist yet (the common case, the
// very first run), a permissions problem, a corrupted file from an
// interrupted write before saveDevelState existed — comes back as an
// empty map rather than an error: remembering settings is a
// convenience layered on top of `crust develop` actually working,
// never a precondition for it starting at all.
func loadDevelState() map[string]develState {
	path, err := develStatePath()
	if err != nil {
		return map[string]develState{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]develState{}
	}
	var m map[string]develState
	if json.Unmarshal(data, &m) != nil || m == nil {
		return map[string]develState{}
	}
	return m
}

// saveDevelState updates absPath's remembered settings and writes the
// whole map back out. Writes to a temp file in the same directory and
// renames it into place — os.Rename is atomic for a same-filesystem
// rename on every platform Go supports, and the temp file lives right
// next to the real one specifically to guarantee that — so a crash or
// interrupt mid-write can never leave develop_state.json
// half-written and unparseable for every future run.
func saveDevelState(absPath string, s develState) error {
	path, err := develStatePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	m := loadDevelState()
	m[absPath] = s

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// saveDevelStateBestEffort persists path's store/input/run-all
// selection for next time. Best-effort: a write failure (disk full,
// permissions) shouldn't interrupt or fail the run itself, so any
// error here is silently dropped — remembering settings is a
// convenience layered on top of running the program, never a
// precondition for it.
func saveDevelStateBestEffort(path, store, input string, runAll bool) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	_ = saveDevelState(abs, develState{Store: store, Input: input, RunAll: runAll})
}

// applySavedStore fills in opts.Store from path's remembered settings
// when the caller didn't already pass an explicit --store — an
// explicit flag always wins over a remembered default, the same
// precedence any "remember my last choice" feature gives an explicit
// override. Applied before the very first recording is built
// (runDebug), so both --plain and the TUI's initial Time/Memory/
// Stepper tabs already reflect the last store used, not just the Run
// tab after it's re-run once.
func applySavedStore(path string, opts debugOptions) debugOptions {
	if opts.Store != "" {
		return opts
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return opts
	}
	if saved, ok := loadDevelState()[abs]; ok {
		opts.Store = saved.Store
	}
	return opts
}

// restoreRunInput returns a runInputModel pre-filled with path's
// remembered input-file setting, cursor at the end — or the zero
// value if nothing's been remembered yet, same as before this feature
// existed.
func restoreRunInput(path string) runInputModel {
	abs, err := filepath.Abs(path)
	if err != nil {
		return runInputModel{}
	}
	saved, ok := loadDevelState()[abs]
	if !ok || saved.Input == "" {
		return runInputModel{}
	}
	value := []rune(saved.Input)
	return runInputModel{value: value, cursor: len(value)}
}

// restoreRunAll returns path's remembered "run all stores" toggle
// (debug_run.go), or false — the ordinary single-entry-point
// behavior — if nothing's been remembered yet, same convention
// restoreRunInput/applySavedStore already use.
func restoreRunAll(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	return loadDevelState()[abs].RunAll
}
