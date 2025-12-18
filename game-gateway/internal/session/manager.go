package session

import (
	"log"
	"sync"

	"game-gateway/internal/logger"

	"github.com/gorilla/websocket"
)

type Session struct {
	ID        string
	Conn      *websocket.Conn
	Send      chan []byte
	UserID    int32
	AuthToken string
}

type Manager struct {
	sessions     map[string]*Session
	userSessions map[int32]*Session
	mu           sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		sessions:     make(map[string]*Session),
		userSessions: make(map[int32]*Session),
	}
}

func (m *Manager) Add(s *Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.ID] = s
	logger.Debug(logger.TagSession, "Added session %s", s.ID)
}

func (m *Manager) Bind(userID int32, sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	logger.Debug(logger.TagSession, "Trying to bind UserID %d to Session %s", userID, sessionID)

	s, ok := m.sessions[sessionID]
	if !ok {
		logger.Error(logger.TagSession, "Bind ERROR - Session %s not found!", sessionID)
		return
	}

	s.UserID = userID
	m.userSessions[userID] = s
	logger.Debug(logger.TagSession, "Successfully bound UserID %d to Session %s", userID, sessionID)
}

func (m *Manager) Remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if ok && s.UserID != 0 {
		logger.Debug(logger.TagSession, "Removing Session %s (UserID=%d)", id, s.UserID)
		delete(m.userSessions, s.UserID)
	} else {
		logger.Debug(logger.TagSession, "Removing Session %s (no UserID)", id)
	}
	delete(m.sessions, id)
}

func (m *Manager) Get(id string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[id]
}

func (m *Manager) GetByUserID(userID int32) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := m.userSessions[userID]
	if s != nil {
		log.Printf("SessionManager.GetByUserID: Found UserID %d -> Session %s", userID, s.ID)
	} else {
		logger.Debug(logger.TagSession, "UserID %d NOT FOUND. Current userSessions map has %d entries", userID, len(m.userSessions))
		// Debug: print all entries
		for uid, sess := range m.userSessions {
			log.Printf("  - UserID %d -> Session %s", uid, sess.ID)
		}
	}
	return s
}
