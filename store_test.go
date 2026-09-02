package main

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCreateAndGet(t *testing.T) {
	s := NewStore()

	p, err := s.Create("hello world", "text", 0)
	if err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}
	if p.ID == "" {
		t.Fatal("Create: expected a non-empty id")
	}
	if p.Content != "hello world" {
		t.Fatalf("Create: expected content %q, got %q", "hello world", p.Content)
	}
	if p.Language != "text" {
		t.Fatalf("Create: expected language %q, got %q", "text", p.Language)
	}
	if p.CreatedAt.IsZero() {
		t.Fatal("Create: expected non-zero CreatedAt")
	}
	if p.ExpiresAt != nil {
		t.Fatalf("Create: expected nil ExpiresAt for expiresInSeconds=0, got %v", p.ExpiresAt)
	}

	got, ok := s.Get(p.ID)
	if !ok {
		t.Fatal("Get: expected paste to be found")
	}
	if got.ID != p.ID {
		t.Fatalf("Get: expected id %q, got %q", p.ID, got.ID)
	}
	if got.Content != "hello world" {
		t.Fatalf("Get: expected content %q, got %q", "hello world", got.Content)
	}
}

func TestGetUnknownReturnsFalse(t *testing.T) {
	s := NewStore()
	if _, ok := s.Get("deadbeefdeadbeefdeadbeefdeadbeef"); ok {
		t.Fatal("Get: expected unknown id to return false")
	}
}

func TestExpiry(t *testing.T) {
	s := NewStore()

	p, err := s.Create("secret", "text", 1)
	if err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}
	if p.ExpiresAt == nil {
		t.Fatal("Create: expected ExpiresAt to be set for expiresInSeconds=1")
	}

	if _, ok := s.Get(p.ID); !ok {
		t.Fatal("Get: expected paste to be found before expiry")
	}

	time.Sleep(2 * time.Second)

	if _, ok := s.Get(p.ID); ok {
		t.Fatal("Get: expected paste to be gone after expiry")
	}

	metas := s.List()
	for _, m := range metas {
		if m.ID == p.ID {
			t.Fatal("List: expected expired paste to be absent")
		}
	}
}

func TestExpiryOnlyWhenPositive(t *testing.T) {
	s := NewStore()

	p, err := s.Create("keeps", "text", 0)
	if err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}
	if p.ExpiresAt != nil {
		t.Fatalf("Create: expected nil ExpiresAt for expiresInSeconds=0, got %v", p.ExpiresAt)
	}
}

func TestIDFormat(t *testing.T) {
	s := NewStore()
	seen := make(map[string]bool)

	for i := 0; i < 10; i++ {
		p, err := s.Create("x", "text", 0)
		if err != nil {
			t.Fatalf("Create: unexpected error: %v", err)
		}
		if len(p.ID) != 32 {
			t.Fatalf("ID: expected 32 characters, got %d (%q)", len(p.ID), p.ID)
		}
		raw, err := hex.DecodeString(p.ID)
		if err != nil {
			t.Fatalf("ID: expected valid hex, got %q: %v", p.ID, err)
		}
		if len(raw) != 16 {
			t.Fatalf("ID: expected 16 decoded bytes, got %d", len(raw))
		}
		if seen[p.ID] {
			t.Fatalf("ID: duplicate id generated: %q", p.ID)
		}
		seen[p.ID] = true
	}
}

func TestDelete(t *testing.T) {
	s := NewStore()

	p, err := s.Create("temp", "text", 0)
	if err != nil {
		t.Fatalf("Create: unexpected error: %v", err)
	}

	if !s.Delete(p.ID) {
		t.Fatal("Delete: expected true for existing id")
	}
	if _, ok := s.Get(p.ID); ok {
		t.Fatal("Get: expected paste to be gone after Delete")
	}
	if s.Delete(p.ID) {
		t.Fatal("Delete: expected false for already-deleted id")
	}
}

func TestListSortedDescendingAndNoContent(t *testing.T) {
	s := NewStore()

	ids := make([]string, 0, 3)
	for i := 0; i < 3; i++ {
		p, err := s.Create("c", "text", 0)
		if err != nil {
			t.Fatalf("Create: unexpected error: %v", err)
		}
		ids = append(ids, p.ID)
		time.Sleep(2 * time.Millisecond)
	}

	metas := s.List()
	if len(metas) != 3 {
		t.Fatalf("List: expected 3 entries, got %d", len(metas))
	}

	// Newest first.
	for i := 0; i < len(metas); i++ {
		want := ids[len(ids)-1-i]
		if metas[i].ID != want {
			t.Fatalf("List: expected %q at index %d, got %q", want, i, metas[i].ID)
		}
		if i > 0 && !metas[i-1].CreatedAt.After(metas[i].CreatedAt) {
			t.Fatalf("List: entries not sorted descending at index %d", i)
		}
	}

	// No content field in the serialized metadata.
	data, err := json.Marshal(metas)
	if err != nil {
		t.Fatalf("List: marshaling metadata failed: %v", err)
	}
	if strings.Contains(string(data), `"content"`) {
		t.Fatalf("List: metadata must not contain content, got %s", string(data))
	}
}
