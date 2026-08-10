package aoc

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// These tests never touch adventofcode.com — every one points Client
// at an httptest.Server, same reasoning as fetch_test.go's own header
// comment: no legitimate session cookie to test with, and no load put
// on the real endpoint from automated runs.

func aocPage(article string) string {
	return "<!DOCTYPE html><html><body><main>" + article + "</main></body></html>"
}

func TestSubmitCorrectAnswer(t *testing.T) {
	var gotPath, gotLevel, gotAnswer, gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		r.ParseForm()
		gotLevel = r.FormValue("level")
		gotAnswer = r.FormValue("answer")
		w.Write([]byte(aocPage(`<article><p>That's the right answer! You are one gold star closer to saving your vacation. <a href="/2026/day/3">[Return to Day 3]</a></p></article>`)))
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Session: "sess"}
	got, err := c.Submit(2026, 3, 1, "42")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if got.Outcome != OutcomeCorrect {
		t.Errorf("Outcome = %v, want OutcomeCorrect", got.Outcome)
	}
	if !strings.Contains(got.Message, "right answer") {
		t.Errorf("Message = %q, want it to mention 'right answer'", got.Message)
	}
	if gotPath != "/2026/day/3/answer" {
		t.Errorf("path = %q", gotPath)
	}
	if gotLevel != "1" {
		t.Errorf("level = %q, want 1", gotLevel)
	}
	if gotAnswer != "42" {
		t.Errorf("answer = %q, want 42", gotAnswer)
	}
	if !strings.Contains(gotContentType, "application/x-www-form-urlencoded") {
		t.Errorf("Content-Type = %q", gotContentType)
	}
}

func TestSubmitIncorrectAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(aocPage(`<article><p>That's not the right answer. If you're stuck, make sure you're using the full input data. <a href="/2026/day/3">[Return to Day 3]</a></p></article>`)))
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Session: "sess"}
	got, err := c.Submit(2026, 3, 1, "0")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if got.Outcome != OutcomeIncorrect {
		t.Errorf("Outcome = %v, want OutcomeIncorrect", got.Outcome)
	}
}

func TestSubmitAnswerTooLow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(aocPage(`<article><p>That's not the right answer; your answer is too low. <a href="/2026/day/3">[Return to Day 3]</a></p></article>`)))
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Session: "sess"}
	got, err := c.Submit(2026, 3, 1, "1")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if got.Outcome != OutcomeTooLow {
		t.Errorf("Outcome = %v, want OutcomeTooLow", got.Outcome)
	}
}

func TestSubmitAnswerTooHigh(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(aocPage(`<article><p>That's not the right answer; your answer is too high. <a href="/2026/day/3">[Return to Day 3]</a></p></article>`)))
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Session: "sess"}
	got, err := c.Submit(2026, 3, 1, "999999")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if got.Outcome != OutcomeTooHigh {
		t.Errorf("Outcome = %v, want OutcomeTooHigh", got.Outcome)
	}
}

func TestSubmitRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(aocPage(`<article><p>You gave an answer too recently; you have to wait after submitting an answer before trying again. You have 45s left to wait. <a href="/2026/day/3">[Return to Day 3]</a></p></article>`)))
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Session: "sess"}
	got, err := c.Submit(2026, 3, 1, "42")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if got.Outcome != OutcomeRateLimited {
		t.Errorf("Outcome = %v, want OutcomeRateLimited", got.Outcome)
	}
	if !strings.Contains(got.Message, "45s") {
		t.Errorf("Message = %q, want it to mention the wait time", got.Message)
	}
}

func TestSubmitAlreadySolved(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(aocPage(`<article><p>You don't seem to be solving the right level. Did you already complete it? <a href="/2026/day/3">[Return to Day 3]</a></p></article>`)))
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Session: "sess"}
	got, err := c.Submit(2026, 3, 2, "42")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if got.Outcome != OutcomeAlreadySolved {
		t.Errorf("Outcome = %v, want OutcomeAlreadySolved", got.Outcome)
	}
}

func TestSubmitUnknownResponseIsNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(aocPage(`<article><p>Something AoC has never said before.</p></article>`)))
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Session: "sess"}
	got, err := c.Submit(2026, 3, 1, "42")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if got.Outcome != OutcomeUnknown {
		t.Errorf("Outcome = %v, want OutcomeUnknown", got.Outcome)
	}
	if !strings.Contains(got.Message, "Something AoC has never said before") {
		t.Errorf("Message = %q, want the raw text preserved", got.Message)
	}
}

func TestSubmitBadSessionReturnsHelpfulError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Puzzle inputs differ by user.", http.StatusBadRequest)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Session: "bad"}
	_, err := c.Submit(2026, 3, 1, "42")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "crust login") {
		t.Errorf("got %q, want it to mention `crust login`", err.Error())
	}
}

func TestSubmitServerErrorSurfacesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "something broke", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Session: "ok"}
	_, err := c.Submit(2026, 3, 1, "42")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "something broke") {
		t.Errorf("got %q, want it to include the response body", err.Error())
	}
}

func TestArticleTextStripsNestedTags(t *testing.T) {
	got := articleText(aocPage(`<article><p>Right answer! <a href="/x">[Return]</a> more text</p></article>`))
	want := "Right answer! [Return] more text"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestArticleTextFallsBackWhenNoArticleTag(t *testing.T) {
	got := articleText("<html><body>plain <b>bold</b> text</body></html>")
	want := "plain bold text"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
