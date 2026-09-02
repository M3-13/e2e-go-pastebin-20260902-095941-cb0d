package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeStore implements storeAPI with the behaviour the shared contract
// specifies for Store, so the handlers can be exercised without depending on
// the real Store implementation (which lives in another ticket).
type fakeStore struct {
	pastes map[string]Paste
	seq    int
}

func newFakeStore() *fakeStore {
	return &fakeStore{pastes: make(map[string]Paste)}
}

func (f *fakeStore) Create(content, language string, expiresInSeconds int) (Paste, error) {
	f.seq++
	now := time.Now()
	p := Paste{
		ID:        fmt.Sprintf("%032x", f.seq),
		Content:   content,
		Language:  language,
		CreatedAt: now,
	}
	if expiresInSeconds > 0 {
		exp := now.Add(time.Duration(expiresInSeconds) * time.Second)
		p.ExpiresAt = &exp
	}
	f.pastes[p.ID] = p
	return p, nil
}

func (f *fakeStore) Get(id string) (Paste, bool) {
	p, ok := f.pastes[id]
	if !ok {
		return Paste{}, false
	}
	if p.ExpiresAt != nil && time.Now().After(*p.ExpiresAt) {
		delete(f.pastes, id)
		return Paste{}, false
	}
	return p, true
}

func (f *fakeStore) List() []Metadata {
	now := time.Now()
	metas := make([]Metadata, 0, len(f.pastes))
	for id, p := range f.pastes {
		if p.ExpiresAt != nil && now.After(*p.ExpiresAt) {
			delete(f.pastes, id)
			continue
		}
		metas = append(metas, Metadata{
			ID:        p.ID,
			Language:  p.Language,
			CreatedAt: p.CreatedAt,
			ExpiresAt: p.ExpiresAt,
		})
	}
	return metas
}

func (f *fakeStore) Delete(id string) bool {
	if _, ok := f.pastes[id]; !ok {
		return false
	}
	delete(f.pastes, id)
	return true
}

func newHandlerWithFake() http.Handler {
	return &handler{store: newFakeStore()}
}

func createPaste(t *testing.T, h http.Handler, body string) Paste {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/pastes", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /pastes %q: expected 201, got %d", body, rec.Code)
	}
	var p Paste
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("POST /pastes %q: invalid JSON body: %v", body, err)
	}
	return p
}

func assertErrorJSON(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body is not valid JSON: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("error body must contain only the error field, got %v", body)
	}
	msg, ok := body["error"].(string)
	if !ok {
		t.Fatalf("error field must be a string, got %v", body)
	}
	if msg == "" {
		t.Fatalf("error field must not be empty")
	}
}

func TestCreatePaste(t *testing.T) {
	h := newHandlerWithFake()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/pastes", strings.NewReader(`{"content":"hello","language":"go"}`))
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /pastes: expected 201, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("POST /pastes: expected Content-Type application/json, got %q", ct)
	}
	var p Paste
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("POST /pastes: invalid JSON body: %v", err)
	}
	if p.ID == "" {
		t.Fatalf("POST /pastes: expected a non-empty id")
	}
	if p.Content != "hello" {
		t.Fatalf("POST /pastes: expected content hello, got %q", p.Content)
	}
	if p.Language != "go" {
		t.Fatalf("POST /pastes: expected language go, got %q", p.Language)
	}
}

func TestCreatePasteExpiryField(t *testing.T) {
	h := newHandlerWithFake()

	p := createPaste(t, h, `{"content":"no expiry"}`)
	if p.ExpiresAt != nil {
		t.Fatalf("POST without expires_in_seconds: expected no expires_at, got %v", p.ExpiresAt)
	}

	p = createPaste(t, h, `{"content":"expires","expires_in_seconds":60}`)
	if p.ExpiresAt == nil {
		t.Fatalf("POST with expires_in_seconds: expected expires_at to be set")
	}
}

func TestGetPaste(t *testing.T) {
	h := newHandlerWithFake()
	p := createPaste(t, h, `{"content":"hello"}`)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/pastes/"+p.ID, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /pastes/%s: expected 200, got %d", p.ID, rec.Code)
	}
	var got Paste
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("GET: invalid JSON body: %v", err)
	}
	if got.Content != "hello" {
		t.Fatalf("GET: expected content hello, got %q", got.Content)
	}
}

func TestGetPasteNotFound(t *testing.T) {
	h := newHandlerWithFake()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/pastes/doesnotexist", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET unknown id: expected 404, got %d", rec.Code)
	}
	assertErrorJSON(t, rec)
}

func TestListPastesWithoutContent(t *testing.T) {
	h := newHandlerWithFake()
	createPaste(t, h, `{"content":"secret"}`)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/pastes", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /pastes: expected 200, got %d", rec.Code)
	}
	var metas []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &metas); err != nil {
		t.Fatalf("GET /pastes: invalid JSON body: %v", err)
	}
	if len(metas) != 1 {
		t.Fatalf("GET /pastes: expected 1 paste, got %d", len(metas))
	}
	if _, ok := metas[0]["content"]; ok {
		t.Fatalf("GET /pastes: list must not contain a content field, got %v", metas[0])
	}
}

func TestDeletePaste(t *testing.T) {
	h := newHandlerWithFake()
	p := createPaste(t, h, `{"content":"bye"}`)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/pastes/"+p.ID, nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE /pastes/%s: expected 204, got %d", p.ID, rec.Code)
	}

	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/pastes/"+p.ID, nil))
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("GET after DELETE: expected 404, got %d", getRec.Code)
	}
}

func TestDeletePasteNotFound(t *testing.T) {
	h := newHandlerWithFake()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/pastes/doesnotexist", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("DELETE unknown id: expected 404, got %d", rec.Code)
	}
	assertErrorJSON(t, rec)
}

func TestCreatePasteInvalidJSON(t *testing.T) {
	h := newHandlerWithFake()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/pastes", strings.NewReader(`{invalid`)))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid JSON: expected 400, got %d", rec.Code)
	}
	assertErrorJSON(t, rec)
}

func TestCreatePasteMissingContent(t *testing.T) {
	h := newHandlerWithFake()
	for _, body := range []string{`{}`, `{"content":""}`, `{"language":"go"}`} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/pastes", strings.NewReader(body)))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("missing content %q: expected 400, got %d", body, rec.Code)
		}
		assertErrorJSON(t, rec)
	}
}

func TestCreatePasteBodyTooLarge(t *testing.T) {
	h := newHandlerWithFake()
	big := strings.Repeat("a", maxPasteBodyBytes+1)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/pastes", strings.NewReader(`{"content":"`+big+`"}`)))

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body: expected 413, got %d", rec.Code)
	}
	assertErrorJSON(t, rec)
}

func TestCreatePasteExpires(t *testing.T) {
	h := newHandlerWithFake()
	p := createPaste(t, h, `{"content":"temp","expires_in_seconds":1}`)
	if p.ExpiresAt == nil {
		t.Fatalf("POST with expires_in_seconds: expected expires_at to be set")
	}

	time.Sleep(1100 * time.Millisecond)

	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/pastes/"+p.ID, nil))
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("GET after expiry: expected 404, got %d", getRec.Code)
	}

	listRec := httptest.NewRecorder()
	h.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/pastes", nil))
	var metas []Metadata
	if err := json.Unmarshal(listRec.Body.Bytes(), &metas); err != nil {
		t.Fatalf("GET /pastes after expiry: invalid JSON: %v", err)
	}
	if len(metas) != 0 {
		t.Fatalf("GET /pastes after expiry: expected empty list, got %v", metas)
	}
}
