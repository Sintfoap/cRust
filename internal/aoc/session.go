// Package aoc talks to adventofcode.com: fetching a day's puzzle input
// and remembering the session cookie needed to do so, plus tracking
// how long a day took to solve. Nothing here runs on its own — every
// entry point is opt-in (`crust login`, `crust fetch`, or `crust
// develop`'s auto-fetch, itself only active once a session has
// actually been saved).
package aoc

import (
	"os"
	"path/filepath"
	"strings"
)

// sessionEnvVar overrides the saved session file when set, matching
// this codebase's other env-var override conventions. Handy for CI,
// or anyone who'd rather not have crust write a config file at all.
const sessionEnvVar = "AOC_SESSION"

// sessionDir resolves to os.UserConfigDir() normally; tests override
// it to a temp directory so they never read or write the real user's
// config directory — same pattern as cmd/crust/debug_state.go's
// develStateDir.
var sessionDir = os.UserConfigDir

// sessionPath is where the session cookie is stored, mirroring
// cmd/crust/debug_state.go's develStatePath layout (same crust/
// subdirectory under the user's config dir). A dedicated file rather
// than folding it into develop_state.json: a session cookie is a
// login credential and gets its own 0600 file, never bundled in next
// to 0644 per-file editor settings.
func sessionPath() (string, error) {
	dir, err := sessionDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "crust", "session"), nil
}

// LoadSession returns the saved AoC session cookie: $AOC_SESSION if
// set, otherwise whatever SaveSession (crust login) last wrote. ok is
// false if neither is available, which every caller treats as "the
// fetch/auto-fetch feature is simply off" rather than an error.
func LoadSession() (session string, ok bool) {
	if v := strings.TrimSpace(os.Getenv(sessionEnvVar)); v != "" {
		return v, true
	}
	path, err := sessionPath()
	if err != nil {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	session = strings.TrimSpace(string(data))
	if session == "" {
		return "", false
	}
	return session, true
}

// SaveSession writes session to the config file (creating its
// directory if needed) with 0600 permissions — this is a login
// credential for the caller's own AoC account, not ordinary config.
func SaveSession(session string) error {
	path, err := sessionPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(session)+"\n"), 0o600)
}
