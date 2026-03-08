package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUploadFormHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/upload/", nil)
	rr := httptest.NewRecorder()

	uploadFormHandler(rr, req, nil)

	if rr.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", rr.Code, http.StatusOK)
	}
	// Check for a basic indicator of the form
	if !contains(rr.Body.String(), "<form") {
		t.Errorf("body does not contain form tag")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[0:len(substr)] == substr || len(s) > len(substr) && contains(s[1:], substr)
}
