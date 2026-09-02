package main

import "sync"

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
	return Paste{}, nil
}

// Get returns the paste with the given id and whether it was found.
func (s *Store) Get(id string) (Paste, bool) {
	return Paste{}, false
}

// List returns the metadata of all stored pastes.
func (s *Store) List() []Metadata {
	return nil
}

// Delete removes the paste with the given id and reports whether it existed.
func (s *Store) Delete(id string) bool {
	return false
}
