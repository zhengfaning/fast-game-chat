package session

import (
	"hash/fnv"
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

// Sharded map for better lock granularity (Phase 2 optimization)
const shardCount = 32

type shard struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

type userShard struct {
	sessions map[int32]*Session
	mu       sync.RWMutex
}

type Manager struct {
	sessionShards [shardCount]*shard
	userShards    [shardCount]*userShard
}

func NewManager() *Manager {
	m := &Manager{}
	for i := 0; i < shardCount; i++ {
		m.sessionShards[i] = &shard{
			sessions: make(map[string]*Session),
		}
		m.userShards[i] = &userShard{
			sessions: make(map[int32]*Session),
		}
	}
	return m
}

// getSessionShard returns the shard for a given session ID
func (m *Manager) getSessionShard(id string) *shard {
	h := fnv.New32a()
	h.Write([]byte(id))
	return m.sessionShards[h.Sum32()%shardCount]
}

// getUserShard returns the shard for a given user ID
func (m *Manager) getUserShard(userID int32) *userShard {
	// Use the userID directly for hashing to distribute evenly
	return m.userShards[uint32(userID)%shardCount]
}

func (m *Manager) Add(s *Session) {
	shard := m.getSessionShard(s.ID)
	shard.mu.Lock()
	shard.sessions[s.ID] = s
	shard.mu.Unlock()
	logger.Debug(logger.TagSession, "Added session %s", s.ID)
}

func (m *Manager) Bind(userID int32, sessionID string) {
	logger.Debug(logger.TagSession, "Trying to bind UserID %d to Session %s", userID, sessionID)

	// Get session from session shard
	sessShard := m.getSessionShard(sessionID)
	sessShard.mu.RLock()
	s, ok := sessShard.sessions[sessionID]
	sessShard.mu.RUnlock()

	if !ok {
		logger.Error(logger.TagSession, "Bind ERROR - Session %s not found!", sessionID)
		return
	}

	// Update session UserID (no lock needed, it's a simple field assignment)
	s.UserID = userID

	// Store in user shard
	userShard := m.getUserShard(userID)
	userShard.mu.Lock()
	userShard.sessions[userID] = s
	userShard.mu.Unlock()

	logger.Debug(logger.TagSession, "Successfully bound UserID %d to Session %s", userID, sessionID)
}

func (m *Manager) Remove(id string) {
	shard := m.getSessionShard(id)
	shard.mu.Lock()
	s, ok := shard.sessions[id]
	if ok {
		if s.UserID != 0 {
			logger.Debug(logger.TagSession, "Removing Session %s (UserID=%d)", id, s.UserID)

			// Remove from user shard
			userShard := m.getUserShard(s.UserID)
			userShard.mu.Lock()
			delete(userShard.sessions, s.UserID)
			userShard.mu.Unlock()
		} else {
			logger.Debug(logger.TagSession, "Removing Session %s (no UserID)", id)
		}
		delete(shard.sessions, id)
	}
	shard.mu.Unlock()
}

func (m *Manager) Get(id string) *Session {
	shard := m.getSessionShard(id)
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	return shard.sessions[id]
}

func (m *Manager) GetByUserID(userID int32) *Session {
	shard := m.getUserShard(userID)
	shard.mu.RLock()
	s := shard.sessions[userID]
	shard.mu.RUnlock()

	if s != nil {
		log.Printf("SessionManager.GetByUserID: Found UserID %d -> Session %s", userID, s.ID)
		return s
	}

	logger.Debug(logger.TagSession, "UserID %d NOT FOUND", userID)
	// Debug: print all entries across all shards
	count := 0
	for i := 0; i < shardCount; i++ {
		sh := m.userShards[i]
		sh.mu.RLock()
		for uid, sess := range sh.sessions {
			log.Printf("  - UserID %d -> Session %s", uid, sess.ID)
			count++
		}
		sh.mu.RUnlock()
	}
	logger.Debug(logger.TagSession, "Current userSessions map has %d entries across %d shards", count, shardCount)

	return nil
}
