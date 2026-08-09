package aoc

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// defaultBaseURL is adventofcode.com's own site root. Client.BaseURL
// overrides it purely so tests can point at an httptest.Server
// instead — this package's own tests never make a real request to
// adventofcode.com, both because there's no legitimate session cookie
// to test with here and to avoid putting any load on the real
// endpoint from automated test runs.
const defaultBaseURL = "https://adventofcode.com"

// userAgent identifies crust to AoC's server, per the site operator's
// own published request that automated tools identify themselves so
// a misbehaving script can be traced back to its source. Points at
// this repo, the most useful contact info available here.
const userAgent = "github.com/Sintfoap/cRust crust-fetch (contact via https://github.com/Sintfoap/cRust)"

// Client fetches puzzle input from adventofcode.com.
type Client struct {
	// BaseURL defaults to defaultBaseURL when empty; only ever
	// overridden in tests, to point at an httptest.Server.
	BaseURL string
	// Session is the "session" cookie value from a logged-in AoC
	// account — the site's entire auth model for this endpoint.
	Session string
	// HTTPClient defaults to a client with a short timeout when nil.
	HTTPClient *http.Client
}

func (c *Client) baseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return defaultBaseURL
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 10 * time.Second}
}

// FetchInput retrieves year/day's puzzle input exactly as the
// authenticated account would see it at
// adventofcode.com/<year>/day/<day>/input, byte for byte — including
// a trailing newline if AoC's own response has one. Nothing here
// trims or reformats the body; a puzzle's expected input format is
// AoC's call, not crust's.
func (c *Client) FetchInput(year, day int) ([]byte, error) {
	url := fmt.Sprintf("%s/%d/day/%d/input", c.baseURL(), year, day)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.AddCookie(&http.Cookie{Name: "session", Value: c.Session})

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching input: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading input response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return body, nil
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden:
		return nil, fmt.Errorf("adventofcode.com rejected the session cookie (%s) — run `crust login` again with a fresh one", resp.Status)
	case http.StatusNotFound:
		return nil, fmt.Errorf("day %d, %d isn't unlocked yet (or doesn't exist)", day, year)
	default:
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("adventofcode.com returned %s: %s", resp.Status, msg)
	}
}
