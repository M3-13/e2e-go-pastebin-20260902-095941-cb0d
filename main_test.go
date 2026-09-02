package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestHandler() http.Handler {
	return NewHandler(NewStore())
}

func TestHealthOK(t *testing.T) {
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /health: expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("GET /health: expected Content-Type application/json, got %q", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("GET /health: invalid JSON body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("GET /health: expected status ok, got %q", body["status"])
	}
}

func TestMethodNotAllowedOnPastes(t *testing.T) {
	h := newTestHandler()
	for _, method := range []string{http.MethodPut, http.MethodPatch} {
		req := httptest.NewRequest(method, "/pastes", nil)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s /pastes: expected 405, got %d", method, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("%s /pastes: expected Content-Type application/json, got %q", method, ct)
		}
		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s /pastes: invalid JSON body: %v", method, err)
		}
		if _, ok := body["error"]; !ok {
			t.Fatalf("%s /pastes: expected an error field, got %v", method, body)
		}
	}
}

func TestTrailingSlashNotFound(t *testing.T) {
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/pastes/", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /pastes/: expected 404, got %d", rec.Code)
	}
}

func TestUnknownPathNotFound(t *testing.T) {
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/nope", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /nope: expected 404, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("GET /nope: expected Content-Type application/json, got %q", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("GET /nope: invalid JSON body: %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Fatalf("GET /nope: expected an error field, got %v", body)
	}
}
