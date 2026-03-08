package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLogHandler(t *testing.T) {
	lc := newLogCache(10)
	fmt.Fprintf(lc, "test log message 1\n")
	fmt.Fprintf(lc, "test log message 2\n")

	s := &site{
		LogCache: lc,
	}

	req := httptest.NewRequest("GET", "/log/", nil)
	rr := httptest.NewRecorder()

	logHandler(rr, req, s)

	if rr.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "test log message 1") {
		t.Errorf("body does not contain 'test log message 1'")
	}
	if !strings.Contains(body, "test log message 2") {
		t.Errorf("body does not contain 'test log message 2'")
	}
}

