// README: In-memory session store for the ride assistant. Will be replaced by Postgres later.
package rideassistant

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

const sessionTTL = 15 * time.Minute

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session expired")
)

// Store is a concurrency-safe in-memory session store.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session       // sessionID → session
	byUser   map[string]string         // userID → active sessionID
}

// NewStore creates an empty session store.
func NewStore() *Store {
	return &Store{
		sessions: make(map[string]*Session),
		byUser:   make(map[string]string),
	}
}

// GetActiveSessionByUserID returns the user's active (non-expired) session, if any.
func (s *Store) GetActiveSessionByUserID(userID string) (*Session, error) {
	s.mu.RLock()
	sid, ok := s.byUser[userID]
	s.mu.RUnlock()
	if !ok {
		return nil, nil
	}
	return s.getIfValid(sid)
}

// GetSession returns a session by ID if it is valid and not expired.
func (s *Store) GetSession(id string) (*Session, error) {
	return s.getIfValid(id)
}

// CreateSession creates a new session for the given user.
// Any previous active session for that user is implicitly cancelled.
func (s *Store) CreateSession(userID string) *Session {
	now := time.Now()
	sess := &Session{
		ID:        newSessionID(),
		UserID:    userID,
		Stage:     StageCollecting,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.mu.Lock()
	// Cancel previous session if it exists.
	if prevID, ok := s.byUser[userID]; ok {
		if prev, found := s.sessions[prevID]; found {
			prev.Stage = StageCancelled
		}
	}
	s.sessions[sess.ID] = sess
	s.byUser[userID] = sess.ID
	s.mu.Unlock()

	return sess
}

// UpdateSession persists changes to a session. Caller must hold no lock.
func (s *Store) UpdateSession(sess *Session) {
	sess.UpdatedAt = time.Now()
	s.mu.Lock()
	s.sessions[sess.ID] = sess
	s.mu.Unlock()
}

// CompleteSession marks a session as completed.
func (s *Store) CompleteSession(id string) error {
	return s.setStage(id, StageCompleted)
}

// CancelSession marks a session as cancelled.
func (s *Store) CancelSession(id string) error {
	return s.setStage(id, StageCancelled)
}

// ---------------------------------------------------------------------------
// Internals
// ---------------------------------------------------------------------------

func (s *Store) getIfValid(id string) (*Session, error) {
	s.mu.RLock()
	sess, ok := s.sessions[id]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrSessionNotFound
	}
	if time.Since(sess.UpdatedAt) > sessionTTL {
		// Lazy-expire: mark cancelled so the slot is freed.
		_ = s.CancelSession(id)
		return nil, ErrSessionExpired
	}
	if sess.Stage == StageCancelled || sess.Stage == StageCompleted {
		return nil, nil
	}
	return sess, nil
}

func (s *Store) setStage(id string, stage string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		return ErrSessionNotFound
	}
	sess.Stage = stage
	sess.UpdatedAt = time.Now()
	// Clear user→session mapping so a new session can be created.
	if stage == StageCancelled || stage == StageCompleted {
		if s.byUser[sess.UserID] == id {
			delete(s.byUser, sess.UserID)
		}
	}
	return nil
}

func newSessionID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return "sess_" + hex.EncodeToString(b)
}
