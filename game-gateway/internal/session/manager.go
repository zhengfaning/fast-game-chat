package session

import (
	"log"

	"game-gateway/internal/logger"

	"github.com/gorilla/websocket"
	cmap "github.com/orcaman/concurrent-map/v2"
)

type Session struct {
	ID        string
	Conn      *websocket.Conn
	Send      chan []byte
	UserID    int32
	AuthToken string
}

// Manager using lock-free concurrent map (Phase 3 optimization)
type Manager struct {
	sessions     cmap.ConcurrentMap[string, *Session] // SessionID -> Session
	userSessions cmap.ConcurrentMap[int32, *Session]  // UserID -> Session
}

func NewManager() *Manager {
	return &Manager{
		sessions: cmap.New[*Session](),
		userSessions: cmap.NewWithCustomShardingFunction[int32, *Session](func(key int32) uint32 {
			return uint32(key) // Simple hash for int32
		}),
	}
}

func (m *Manager) Add(s *Session) {
	m.sessions.Set(s.ID, s)
	logger.Debug(logger.TagSession, "Added session %s", s.ID)
}

func (m *Manager) Bind(userID int32, sessionID string) {
	logger.Debug(logger.TagSession, "Trying to bind UserID %d to Session %s", userID, sessionID)

	s, ok := m.sessions.Get(sessionID)
	if !ok {
		logger.Error(logger.TagSession, "Bind ERROR - Session %s not found!", sessionID)
		return
	}

	// Update session UserID (atomic operation on the field level)
	s.UserID = userID
	m.userSessions.Set(userID, s)
	logger.Debug(logger.TagSession, "Successfully bound UserID %d to Session %s", userID, sessionID)
}

func (m *Manager) Remove(id string) {
	s, ok := m.sessions.Get(id)
	if ok {
		if s.UserID != 0 {
			logger.Debug(logger.TagSession, "Removing Session %s (UserID=%d)", id, s.UserID)
			m.userSessions.Remove(s.UserID)
		} else {
			logger.Debug(logger.TagSession, "Removing Session %s (no UserID)", id)
		}
		m.sessions.Remove(id)
	}
}

func (m *Manager) Get(id string) *Session {
	s, _ := m.sessions.Get(id)
	return s
}

func (m *Manager) GetByUserID(userID int32) *Session {
	s, ok := m.userSessions.Get(userID)
	if ok {
		log.Printf("SessionManager.GetByUserID: Found UserID %d -> Session %s", userID, s.ID)
		return s
	}

	logger.Debug(logger.TagSession, "UserID %d NOT FOUND", userID)
	// Debug: print all entries
	count := 0
	for item := range m.userSessions.IterBuffered() {
		log.Printf("  - UserID %d -> Session %s", item.Key, item.Val.ID)
		count++
	}
	logger.Debug(logger.TagSession, "Current userSessions map has %d entries", count)

	return nil
}
