package session

import (
	"errors"
	"regexp"
	"sync"
	"time"
)

const (
	MaxParticipants = 5
	sessionTimeout  = time.Hour
	redirectWindow  = 30 * time.Second
)

// Status represents the lifecycle state of a session.
type Status string

const (
	StatusWaiting Status = "waiting"
	StatusActive  Status = "active"
	StatusEnded   Status = "ended"
)

// View is a snapshot of the session safe to marshal as JSON.
type View struct {
	ID           string          `json:"id"`
	Goal         int             `json:"goal"`
	Public       bool            `json:"public"`
	Status       Status          `json:"status"`
	Participants []ParticipantView `json:"participants"`
}

// ParticipantView is a snapshot of a participant safe to marshal as JSON.
type ParticipantView struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	WordCount   int    `json:"wordCount"`
	Connected   bool   `json:"connected"`
	Completed   bool   `json:"completed"`
	JoinOrder   int    `json:"joinOrder"`
	FinishOrder int    `json:"finishOrder"`
}

// Participant holds the server-side state for one user in a session.
type Participant struct {
	ID          string
	Username    string
	WordCount   int
	Connected   bool
	Completed   bool
	JoinOrder   int
	FinishOrder int // 0 = not finished; 1 = first, 2 = second, etc.
	JoinedAt    time.Time
	CompletedAt *time.Time

	sendCh chan []byte // nil when disconnected
}

// Session is the in-memory state for one write-race session.
type Session struct {
	ID     string
	Goal   int
	Public bool
	Status Status

	participants map[string]*Participant // keyed by participantId
	finishCount  int

	doneCh   chan struct{}
	doneOnce sync.Once

	mu sync.Mutex
}

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9 _-]+$`)

// ValidateUsername returns an error message if the username is invalid, or empty string if valid.
func ValidateUsername(username string) string {
	if username == "" {
		return "username is required"
	}
	if !usernameRe.MatchString(username) {
		return "username may only contain letters, numbers, spaces, hyphens, and underscores"
	}
	return ""
}

// View returns a snapshot of the session safe to marshal as JSON.
// Caller must NOT hold s.mu.
func (s *Session) View() View {
	s.mu.Lock()
	defer s.mu.Unlock()

	participants := make([]ParticipantView, 0, len(s.participants))
	for _, p := range s.participants {
		participants = append(participants, ParticipantView{
			ID:          p.ID,
			Username:    p.Username,
			WordCount:   p.WordCount,
			Connected:   p.Connected,
			Completed:   p.Completed,
			JoinOrder:   p.JoinOrder,
			FinishOrder: p.FinishOrder,
		})
	}

	return View{
		ID:           s.ID,
		Goal:         s.Goal,
		Public:       s.Public,
		Status:       s.Status,
		Participants: participants,
	}
}

// AddParticipant creates and registers a new participant.
// Caller must NOT hold s.mu.
func (s *Session) AddParticipant(username string, sendCh chan []byte) (*Participant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Status == StatusEnded {
		return nil, errors.New("session has ended")
	}
	if s.Status == StatusActive {
		return nil, errors.New("session has already started")
	}
	if len(s.participants) >= MaxParticipants {
		return nil, errors.New("session is full")
	}
	for _, p := range s.participants {
		if p.Username == username {
			return nil, errors.New("username already taken")
		}
	}

	p := &Participant{
		ID:        newID(),
		Username:  username,
		Connected: true,
		JoinOrder: len(s.participants) + 1,
		JoinedAt:  time.Now(),
		sendCh:    sendCh,
	}
	s.participants[p.ID] = p
	return p, nil
}

// Reconnect marks an existing participant as connected.
// Caller must NOT hold s.mu.
func (s *Session) Reconnect(participantID string, sendCh chan []byte) (*Participant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.participants[participantID]
	if !ok {
		return nil, errors.New("participant not found")
	}
	if s.Status == StatusEnded {
		return nil, errors.New("session has ended")
	}
	p.Connected = true
	p.sendCh = sendCh
	return p, nil
}

// Disconnect marks a participant as disconnected, closes their send channel
// (which signals the writePump to exit), and clears the reference.
// Caller must NOT hold s.mu.
func (s *Session) Disconnect(participantID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.participants[participantID]; ok {
		p.Connected = false
		if p.sendCh != nil {
			close(p.sendCh)
			p.sendCh = nil
		}
	}
}

// IsHost reports whether the participant with the given ID is the session host
// (JoinOrder == 1). Returns false if the participant does not exist.
func (s *Session) IsHost(participantID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.participants[participantID]
	return ok && p.JoinOrder == 1
}

// Start transitions the session from waiting to active.
// Returns false if already started or ended.
func (s *Session) Start() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Status != StatusWaiting {
		return false
	}
	s.Status = StatusActive
	return true
}

// EndIfHostLeft ends the session if p is the host and the session is still
// waiting. Returns true if the session was ended. Does NOT broadcast.
func (s *Session) EndIfHostLeft(p *Participant) bool {
	s.mu.Lock()
	isHost := p.JoinOrder == 1
	isWaiting := s.Status == StatusWaiting
	s.mu.Unlock()
	if isHost && isWaiting {
		s.End()
		return true
	}
	return false
}

// End transitions the session to ended. Idempotent.
func (s *Session) End() {
	s.mu.Lock()
	alreadyEnded := s.Status == StatusEnded
	s.Status = StatusEnded
	s.mu.Unlock()

	if !alreadyEnded {
		s.doneOnce.Do(func() { close(s.doneCh) })
	}
}

// UpdateWordCount sets a participant's word count and checks goal completion.
// Returns the updated participant view and whether the whole session just ended.
// Caller must NOT hold s.mu.
func (s *Session) UpdateWordCount(participantID string, count int) (ParticipantView, bool) {
	s.mu.Lock()

	p, ok := s.participants[participantID]
	if !ok || s.Status != StatusActive {
		s.mu.Unlock()
		return ParticipantView{}, false
	}

	p.WordCount = count

	if !p.Completed && count >= s.Goal {
		p.Completed = true
		s.finishCount++
		now := time.Now()
		p.CompletedAt = &now
		p.FinishOrder = s.finishCount
	}

	view := ParticipantView{
		ID:          p.ID,
		Username:    p.Username,
		WordCount:   p.WordCount,
		Connected:   p.Connected,
		Completed:   p.Completed,
		JoinOrder:   p.JoinOrder,
		FinishOrder: p.FinishOrder,
	}

	allDone := true
	for _, participant := range s.participants {
		if !participant.Completed {
			allDone = false
			break
		}
	}
	if allDone {
		s.Status = StatusEnded
	}
	s.mu.Unlock()

	if allDone {
		s.doneOnce.Do(func() { close(s.doneCh) })
		return view, true
	}
	return view, false
}

// Broadcast sends msg to every connected participant.
func (s *Session) Broadcast(msg []byte) {
	s.BroadcastExcept(msg, "")
}

// BroadcastExcept sends msg to every connected participant except excludeID.
func (s *Session) BroadcastExcept(msg []byte, excludeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.participants {
		if p.ID != excludeID && p.sendCh != nil {
			select {
			case p.sendCh <- msg:
			default:
			}
		}
	}
}

// SendTo sends msg to one specific participant.
func (s *Session) SendTo(participantID string, msg []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.participants[participantID]
	if ok && p.sendCh != nil {
		select {
		case p.sendCh <- msg:
		default:
		}
	}
}
