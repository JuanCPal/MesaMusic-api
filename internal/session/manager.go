package session

import (
	"sync"
	"time"

	"musica-colaborativa-api/internal/queue"
)

type Manager struct {
	mu             sync.Mutex
	sessions       map[string]*Session
	backupPlaylist []string
}

func NewManager(backupPlaylist []string) *Manager {
	return &Manager{
		sessions:       make(map[string]*Session),
		backupPlaylist: append([]string(nil), backupPlaylist...),
	}
}

func (m *Manager) Create(name string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	session := &Session{
		ID:        genID(),
		Name:      name,
		CreatedAt: time.Now(),
		Status:    StatusActive,
		Queue:     queue.NewManager(m.backupPlaylist),
	}

	m.sessions[session.ID] = session
	return session
}

func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[id]
	return session, ok
}

func (m *Manager) End(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[id]
	if !ok {
		return false
	}

	session.Status = StatusEnded
	return true
}
