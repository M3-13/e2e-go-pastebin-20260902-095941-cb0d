package main

import (
	"crypto/rand"
	"encoding/hex"
	"sort"
	"sync"
	"time"
)

// Store is an in-memory paste store guarded by a read-write mutex.
type Store struct {
	mu     sync.RWMutex
	pastes map[string]Paste
}

// NewStore returns an empty Store.
func NewStore() *Store {
	return &Store{
		pastes: make(map[string]Paste),
	}
}

// Create stores a new paste and returns it.
func (s *Store) Create(content, language string, expiresInSeconds int) (Paste, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return Paste{}, err
	}

	now := time.Now().UTC()
	p := Paste{
		ID:        hex.EncodeToString(buf),
		Content:   content,
		Language:  language,
		CreatedAt: now,
	}
	if expiresInSeconds > 0 {
		expires := now.Add(time.Duration(expiresInSeconds) * time.Second)
		p.ExpiresAt = &expires
	}

	s.mu.Lock()
	s.pastes[p.ID] = p
	s.mu.Unlock()

	return p, nil
}

// Get returns the paste with the given id and whether it was found. Expired
// entries are removed from the map on access.
func (s *Store) Get(id string) (Paste, bool) {
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.pastes[id]
	if !ok {
		return Paste{}, false
	}
	if p.ExpiresAt != nil && !p.ExpiresAt.After(now) {
		delete(s.pastes, id)
		return Paste{}, false
	}
	return p, true
}

// List returns the metadata of all non-expired pastes, sorted by CreatedAt
// descending, without content.
func (s *Store) List() []Metadata {
	now := time.Now().UTC()

	s.mu.RLock()
	metas := make([]Metadata, 0, len(s.pastes))
	for _, p := range s.pastes {
		if p.ExpiresAt != nil && !p.ExpiresAt.After(now) {
			continue
		}
		metas = append(metas, Metadata{
			ID:        p.ID,
			Language:  p.Language,
			CreatedAt: p.CreatedAt,
			ExpiresAt: p.ExpiresAt,
		})
	}
	s.mu.RUnlock()

	sort.Slice(metas, func(i, j int) bool {
		return metas[i].CreatedAt.After(metas[j].CreatedAt)
	})
	return metas
}

// Delete removes the paste with the given id and reports whether it existed.
func (s *Store) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.pastes[id]; !ok {
		return false
	}
	delete(s.pastes, id)
	return true
}
