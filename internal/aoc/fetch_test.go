package aoc

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// These tests never touch adventofcode.com — every one points Client
// at an httptest.Server, both because there's no legitimate session
// cookie available to test against the real site here and to keep
// automated test runs from putting any load on it at all.

func TestFetchInputSuccessReturnsBodyAndSetsHeaders(t *testing.T) {
	var gotCookie, gotUA, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotUA = r.Header.Get("User-Agent")
		if c, err := r.Cookie("session"); err == nil {
			gotCookie = c.Value
		}
		w.Write([]byte("1,2,3\n4,5,6\n"))
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Session: "my-session-cookie"}
	got, err := c.FetchInput(2026, 3)
	if err != nil {
		t.Fatalf("FetchInput: %v", err)
	}
	if string(got) != "1,2,3\n4,5,6\n" {
		t.Fatalf("got %q", got)
	}
	if gotPath != "/2026/day/3/input" {
		t.Fatalf("got path %q", gotPath)
	}
	if gotCookie != "my-session-cookie" {
		t.Fatalf("got cookie %q", gotCookie)
	}
	if gotUA == "" || !strings.Contains(gotUA, "crust") {
		t.Fatalf("got User-Agent %q, want it to identify crust", gotUA)
	}
}

func TestFetchInputBadSessionReturnsHelpfulError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Puzzle inputs differ by user.", http.StatusBadRequest)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Session: "bad"}
	_, err := c.FetchInput(2026, 1)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "crust login") {
		t.Fatalf("got %q, want it to mention `crust login`", err.Error())
	}
}

func TestFetchInputNotFoundMentionsUnlock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Session: "ok"}
	_, err := c.FetchInput(2026, 25)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "unlocked") {
		t.Fatalf("got %q, want it to mention 'unlocked'", err.Error())
	}
}

func TestFetchInputServerErrorSurfacesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "something broke", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Session: "ok"}
	_, err := c.FetchInput(2026, 1)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "something broke") {
		t.Fatalf("got %q, want it to include the response body", err.Error())
	}
}
