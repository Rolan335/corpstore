package telegram

import "sync"

// Session holds telegram chat authentication state.
type Session struct {
	UserID string
	Mode   SessionMode
	Temp   SessionTemp
}

type SessionMode string

const (
	ModeNone               SessionMode = ""
	ModeRegisterUsername   SessionMode = "register_username"
	ModeRegisterPassword   SessionMode = "register_password"
	ModeLoginUsername      SessionMode = "login_username"
	ModeLoginPassword      SessionMode = "login_password"
	ModeShareUsername      SessionMode = "share_username"
)

type SessionTemp struct {
	Username string
	FileID   string
}

// SessionStore persists sessions for telegram chats.
type SessionStore interface {
	Set(chatID int64, s Session)
	Get(chatID int64) (Session, bool)
	Clear(chatID int64)
}

// InMemorySessionStore is a simple in-memory session store.
type InMemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[int64]Session
}

func NewInMemorySessionStore() *InMemorySessionStore {
	return &InMemorySessionStore{
		sessions: make(map[int64]Session),
	}
}

func (s *InMemorySessionStore) Set(chatID int64, sess Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[chatID] = sess
}

func (s *InMemorySessionStore) Get(chatID int64) (Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[chatID]
	return sess, ok
}

func (s *InMemorySessionStore) Clear(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, chatID)
}
