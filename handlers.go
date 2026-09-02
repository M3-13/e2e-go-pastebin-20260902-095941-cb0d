package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

// maxPasteBodyBytes caps the request body accepted by POST /pastes at 1 MiB.
const maxPasteBodyBytes = 1 << 20

// pastesPath is the paste collection path, exactly as the sprint contract
// declares it. The single-paste path is pastesPath + "/" + id.
const pastesPath = "/pastes"

// storeAPI is the slice of the Store surface the handlers depend on.
type storeAPI interface {
	Create(content, language string, expiresInSeconds int) (Paste, error)
	Get(id string) (Paste, bool)
	List() []Metadata
	Delete(id string) bool
}

// createPasteRequest is the JSON body accepted by POST /pastes.
type createPasteRequest struct {
	Content          string `json:"content"`
	Language         string `json:"language"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
}

type handler struct {
	store storeAPI
}

// NewHandler builds the http.Handler for the pastebin API.
func NewHandler(s *Store) http.Handler {
	return &handler{store: s}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/health":
		h.handleHealth(w, r)
	case r.URL.Path == pastesPath:
		h.routePastes(w, r)
	case strings.HasPrefix(r.URL.Path, pastesPath+"/"):
		id := strings.TrimPrefix(r.URL.Path, pastesPath+"/")
		if id == "" || strings.Contains(id, "/") {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		h.routePasteByID(w, r)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

func (h *handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handler) routePastes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.handleCreatePaste(w, r)
	case http.MethodGet:
		h.handleListPastes(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *handler) routePasteByID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleGetPaste(w, r)
	case http.MethodDelete:
		h.handleDeletePaste(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *handler) handleCreatePaste(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxPasteBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var req createPasteRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	expires := 0
	if req.ExpiresInSeconds > 0 {
		expires = req.ExpiresInSeconds
	}

	paste, err := h.store.Create(req.Content, req.Language, expires)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, paste)
}

func (h *handler) handleGetPaste(w http.ResponseWriter, r *http.Request) {
	paste, ok := h.store.Get(pasteID(r))
	if !ok {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	writeJSON(w, http.StatusOK, paste)
}

func (h *handler) handleListPastes(w http.ResponseWriter, r *http.Request) {
	metas := h.store.List()
	if metas == nil {
		metas = []Metadata{}
	}
	writeJSON(w, http.StatusOK, metas)
}

func (h *handler) handleDeletePaste(w http.ResponseWriter, r *http.Request) {
	if !h.store.Delete(pasteID(r)) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}

// pasteID extracts the paste id from a /pastes/{id} request path.
func pasteID(r *http.Request) string {
	return strings.TrimPrefix(r.URL.Path, pastesPath+"/")
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
