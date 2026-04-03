package session

import (
	"sync"
	"time"
)

// LobbyEntry is a public snapshot of a session for the lobby listing.
type LobbyEntry struct {
	ID           string `json:"id"`
	Goal         int    `json:"goal"`
	HostUsername string `json:"hostUsername"`
	WriterCount  int    `json:"writerCount"`
}

// Manager holds all active sessions.
type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewManager creates a new empty Manager.
func NewManager() *Manager {
	return &Manager{sessions: make(map[string]*Session)}
}

func (m *Manager) Create(goal int, hostUsername string, public bool) (*Session, string, error) {
	hostID := newID()
	host := &Participant{
		ID:        hostID,
		Username:  hostUsername,
		Connected: false, // not connected until WebSocket opens
		JoinOrder: 1,
		JoinedAt:  time.Now(),
	}

	s := &Session{
		ID:           newID(),
		Goal:         goal,
		Public:       public,
		Status:       StatusWaiting,
		participants: map[string]*Participant{hostID: host},
		doneCh:       make(chan struct{}),
	}

	m.mu.Lock()
	m.sessions[s.ID] = s
	m.mu.Unlock()

	go func() {
		select {
		case <-time.After(sessionTimeout):
			s.End()
			s.Broadcast([]byte(`{"type":"session_ended"}`))
		case <-s.doneCh:
			// Session ended by another means. Wait a short window so that the
			// host can POST a redirect before we remove the session from the
			// manager (participants' WS connections are still open during this
			// time and can receive a session_redirect message).
		}
		time.Sleep(redirectWindow)
		m.Remove(s.ID)
	}()

	return s, hostID, nil
}

func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.RLock()
	s, ok := m.sessions[id]
	m.mu.RUnlock()
	return s, ok
}

func (m *Manager) Remove(id string) {
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
}

// PublicWaiting returns lobby entries for all public, waiting, non-full sessions.
func (m *Manager) PublicWaiting() []LobbyEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]LobbyEntry, 0)
	for _, s := range m.sessions {
		s.mu.Lock()
		if !s.Public || s.Status != StatusWaiting || len(s.participants) >= MaxParticipants {
			s.mu.Unlock()
			continue
		}
		var hostUsername string
		for _, p := range s.participants {
			if p.JoinOrder == 1 {
				hostUsername = p.Username
				break
			}
		}
		result = append(result, LobbyEntry{
			ID:           s.ID,
			Goal:         s.Goal,
			HostUsername: hostUsername,
			WriterCount:  len(s.participants),
		})
		s.mu.Unlock()
	}
	return result
}
