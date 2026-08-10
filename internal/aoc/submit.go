package aoc

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// SubmitOutcome classifies adventofcode.com's response to a submitted
// answer, parsed out of the response page's own wording — AoC has no
// structured (JSON) submission API, only this HTML page, so Submit
// reads the same sentence a browser user would see.
type SubmitOutcome int

const (
	// OutcomeUnknown means the response didn't match any of the known
	// AoC response phrasings below — still not an error (the request
	// itself succeeded), just something Submit's caller should show
	// Message for directly rather than trust a classification of.
	OutcomeUnknown SubmitOutcome = iota
	OutcomeCorrect
	OutcomeIncorrect
	// OutcomeTooLow/OutcomeTooHigh are OutcomeIncorrect's own two
	// common special cases — AoC often hints which direction to
	// adjust a numeric answer, valuable enough for a binary-search
	// workflow to deserve their own outcomes rather than making every
	// caller re-parse Message for "too low"/"too high" themselves.
	OutcomeTooLow
	OutcomeTooHigh
	// OutcomeRateLimited means AoC's own "you have to wait between
	// submissions" cooldown is active; Message carries the site's own
	// wait-time wording verbatim rather than crust re-parsing and
	// re-formatting a duration out of it.
	OutcomeRateLimited
	// OutcomeAlreadySolved covers both "you already have this star"
	// and "that level isn't unlocked yet" — AoC's own response text
	// doesn't distinguish the two ("You don't seem to be solving the
	// right level..."), so neither does this.
	OutcomeAlreadySolved
)

// SubmitResult is Submit's return value: a classified Outcome plus the
// human-readable Message AoC's own response page actually said, for
// callers that want to show it verbatim (every outcome, not just
// OutcomeUnknown — the exact wording of a "too low"/rate-limit message
// often carries detail Outcome alone doesn't capture).
type SubmitResult struct {
	Outcome SubmitOutcome
	Message string
}

// Submit posts answer as year/day's level (1 or 2, i.e. part1/part2)
// submission, exactly as adventofcode.com/<year>/day/<day>/answer's
// own form does, and classifies the response.
func (c *Client) Submit(year, day, level int, answer string) (*SubmitResult, error) {
	submitURL := fmt.Sprintf("%s/%d/day/%d/answer", c.baseURL(), year, day)
	form := url.Values{"level": {fmt.Sprintf("%d", level)}, "answer": {answer}}

	req, err := http.NewRequest(http.MethodPost, submitURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "session", Value: c.Session})

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("submitting answer: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading submission response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return classifySubmitResponse(string(body)), nil
	case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden:
		return nil, fmt.Errorf("adventofcode.com rejected the session cookie (%s) — run `crust login` again with a fresh one", resp.Status)
	default:
		msg := strings.TrimSpace(articleText(string(body)))
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("adventofcode.com returned %s: %s", resp.Status, msg)
	}
}

// classifySubmitResponse turns the response page's own wording into a
// SubmitResult — plain substring matching, deliberately: the phrases
// below are AoC's own stable, long-standing response text, narrow and
// well-known enough that pulling in an HTML parser (a new dependency,
// against the zero-Go-dependency policy) for what amounts to five
// known sentences would be solving a problem this doesn't have.
func classifySubmitResponse(body string) *SubmitResult {
	msg := articleText(body)
	lower := strings.ToLower(msg)

	switch {
	case strings.Contains(lower, "that's the right answer"):
		return &SubmitResult{Outcome: OutcomeCorrect, Message: msg}
	case strings.Contains(lower, "you gave an answer too recently"):
		return &SubmitResult{Outcome: OutcomeRateLimited, Message: msg}
	case strings.Contains(lower, "don't seem to be solving the right level"):
		return &SubmitResult{Outcome: OutcomeAlreadySolved, Message: msg}
	case strings.Contains(lower, "not the right answer"):
		switch {
		case strings.Contains(lower, "too low"):
			return &SubmitResult{Outcome: OutcomeTooLow, Message: msg}
		case strings.Contains(lower, "too high"):
			return &SubmitResult{Outcome: OutcomeTooHigh, Message: msg}
		default:
			return &SubmitResult{Outcome: OutcomeIncorrect, Message: msg}
		}
	default:
		return &SubmitResult{Outcome: OutcomeUnknown, Message: msg}
	}
}

// articleText pulls the plain text out of the response page's first
// <article>...<p>...</p> — where AoC always puts the one sentence that
// actually matters — stripping any nested tags (the page routinely
// wraps part of the message in <a href="...">Return to Day N</a> or
// similar). Falls back to the whole body, tags stripped, if the
// expected structure isn't found — AoC changing its page layout
// shouldn't turn "wrong answer" into a raw HTML dump.
func articleText(body string) string {
	src := body
	if i := strings.Index(body, "<article>"); i != -1 {
		src = body[i:]
	}
	if i := strings.Index(src, "<p>"); i != -1 {
		src = src[i+len("<p>"):]
		if j := strings.Index(src, "</p>"); j != -1 {
			src = src[:j]
		}
	}
	return strings.TrimSpace(stripTags(src))
}

// stripTags removes everything between < and > (inclusive) — a plain
// scan, not a real HTML parser; good enough for AoC's own simple
// response markup, the same "narrow task, no new dependency" call
// classifySubmitResponse's own doc comment explains.
func stripTags(s string) string {
	var out strings.Builder
	depth := 0
	for _, r := range s {
		switch {
		case r == '<':
			depth++
		case r == '>':
			if depth > 0 {
				depth--
			}
		case depth == 0:
			out.WriteRune(r)
		}
	}
	return out.String()
}
