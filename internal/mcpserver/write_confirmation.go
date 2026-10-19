package mcpserver

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type writeConfirmation struct {
	key     string
	expires time.Time
}
type writeConfirmations struct {
	mu     sync.Mutex
	now    func() time.Time
	tokens map[string]writeConfirmation
}

func newWriteConfirmations(now func() time.Time) *writeConfirmations {
	if now == nil {
		now = time.Now
	}
	return &writeConfirmations{now: now, tokens: make(map[string]writeConfirmation)}
}
func (s *writeConfirmations) issue(key string) (string, error) {
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes[:])
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for token, entry := range s.tokens {
		if !now.Before(entry.expires) {
			delete(s.tokens, token)
		}
	}
	s.tokens[token] = writeConfirmation{key: key, expires: now.Add(2 * time.Minute)}
	return token, nil
}

// consume atomically burns a presented token, including a wrong-key attempt.
// This store is private to delete registration, so the action binding is implicit.
func (s *writeConfirmations) consume(token, key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.tokens[token]
	delete(s.tokens, token)
	return ok && entry.key == key && s.now().Before(entry.expires)
}
