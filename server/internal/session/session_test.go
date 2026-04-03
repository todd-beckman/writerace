package session

import (
	"encoding/json"
	"testing"
)

func newTestSession(goal int, status Status) *Session {
	return &Session{
		ID:           "sess-test",
		Goal:         goal,
		Status:       status,
		participants: make(map[string]*Participant),
		doneCh:       make(chan struct{}),
	}
}

func makeSendCh() chan []byte { return make(chan []byte, 32) }

func addTestParticipant(t *testing.T, session *Session, username string) *Participant {
	t.Helper()
	participant, err := session.AddParticipant(username, makeSendCh())
	if err != nil {
		t.Fatalf("unexpected error adding participant %q: %v", username, err)
	}
	return participant
}

func TestAddParticipant(t *testing.T) {
	testCases := []struct {
		name       string
		setup      func(*Session)
		username   string
		wantErrMsg string
	}{
		{
			name:     "success",
			username: "alice",
		},
		{
			name: "duplicate username",
			setup: func(session *Session) {
				addTestParticipant(t, session, "bob")
			},
			username:   "bob",
			wantErrMsg: "username already taken",
		},
		{
			name: "session full",
			setup: func(session *Session) {
				for i := range MaxParticipants {
					addTestParticipant(t, session, string(rune('a'+i)))
				}
			},
			username:   "extra",
			wantErrMsg: "session is full",
		},
		{
			name:       "session active",
			setup:      func(session *Session) { session.Status = StatusActive },
			username:   "alice",
			wantErrMsg: "session has already started",
		},
		{
			name:       "session ended",
			setup:      func(session *Session) { session.Status = StatusEnded },
			username:   "alice",
			wantErrMsg: "session has ended",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			session := newTestSession(100, StatusWaiting)
			if tc.setup != nil {
				tc.setup(session)
			}

			participant, err := session.AddParticipant(tc.username, makeSendCh())

			if tc.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tc.wantErrMsg)
				}
				if err.Error() != tc.wantErrMsg {
					t.Fatalf("expected error %q, got %q", tc.wantErrMsg, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if participant.Username != tc.username {
				t.Errorf("Username = %q, want %q", participant.Username, tc.username)
			}
			if !participant.Connected {
				t.Error("Connected should be true")
			}
			if participant.JoinOrder != 1 {
				t.Errorf("JoinOrder = %d, want 1", participant.JoinOrder)
			}
		})
	}
}

func TestAddParticipantJoinOrder(t *testing.T) {
	session := newTestSession(100, StatusWaiting)
	names := []string{"alice", "bob", "carol"}
	for i, name := range names {
		participant := addTestParticipant(t, session, name)
		if participant.JoinOrder != i+1 {
			t.Errorf("%s: JoinOrder = %d, want %d", name, participant.JoinOrder, i+1)
		}
	}
}

func TestReconnect(t *testing.T) {
	testCases := []struct {
		name          string
		setup         func(*Session) string // returns the participantID to reconnect
		wantErrMsg    string
		wantConnected bool
	}{
		{
			name: "success",
			setup: func(session *Session) string {
				participant := addTestParticipant(t, session, "alice")
				session.Disconnect(participant.ID)
				return participant.ID
			},
			wantConnected: true,
		},
		{
			name: "participant not found",
			setup: func(session *Session) string {
				return "nonexistent-id"
			},
			wantErrMsg: "participant not found",
		},
		{
			name: "session ended",
			setup: func(session *Session) string {
				participant := addTestParticipant(t, session, "alice")
				session.Status = StatusEnded
				return participant.ID
			},
			wantErrMsg: "session has ended",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			session := newTestSession(100, StatusWaiting)
			id := tc.setup(session)

			participant, err := session.Reconnect(id, makeSendCh())

			if tc.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tc.wantErrMsg)
				}
				if err.Error() != tc.wantErrMsg {
					t.Fatalf("expected error %q, got %q", tc.wantErrMsg, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if participant.Connected != tc.wantConnected {
				t.Errorf("Connected = %v, want %v", participant.Connected, tc.wantConnected)
			}
			if participant.sendCh == nil {
				t.Error("sendCh should be set after reconnect")
			}
		})
	}
}

func TestDisconnect(t *testing.T) {
	session := newTestSession(100, StatusWaiting)
	participant := addTestParticipant(t, session, "alice")

	session.Disconnect(participant.ID)

	if participant.Connected {
		t.Error("Connected should be false after disconnect")
	}
	if participant.sendCh != nil {
		t.Error("sendCh should be nil after disconnect")
	}
}

func TestDisconnectUnknownID(t *testing.T) {
	session := newTestSession(100, StatusWaiting)
	// Should not panic on unknown ID.
	session.Disconnect("nonexistent")
}

func TestStart(t *testing.T) {
	testCases := []struct {
		name       string
		status     Status
		wantResult bool
		wantStatus Status
	}{
		{
			name:       "waiting transitions to active",
			status:     StatusWaiting,
			wantResult: true,
			wantStatus: StatusActive,
		},
		{
			name:       "already active returns false",
			status:     StatusActive,
			wantResult: false,
			wantStatus: StatusActive,
		},
		{
			name:       "ended returns false",
			status:     StatusEnded,
			wantResult: false,
			wantStatus: StatusEnded,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			session := newTestSession(100, tc.status)
			result := session.Start()
			if result != tc.wantResult {
				t.Errorf("Start() = %v, want %v", result, tc.wantResult)
			}
			if session.Status != tc.wantStatus {
				t.Errorf("Status = %q, want %q", session.Status, tc.wantStatus)
			}
		})
	}
}

func TestEnd(t *testing.T) {
	t.Run("transitions to ended and closes doneCh", func(t *testing.T) {
		session := newTestSession(100, StatusActive)
		session.End()
		if session.Status != StatusEnded {
			t.Errorf("Status = %q, want %q", session.Status, StatusEnded)
		}
		select {
		case <-session.doneCh:
			// expected
		default:
			t.Error("doneCh should be closed after End()")
		}
	})

	t.Run("idempotent — second call does not panic", func(t *testing.T) {
		session := newTestSession(100, StatusActive)
		session.End()
		session.End() // must not panic on double-close of doneCh
		if session.Status != StatusEnded {
			t.Errorf("Status = %q, want %q", session.Status, StatusEnded)
		}
	})
}

func TestUpdateWordCount(t *testing.T) {
	t.Run("updates word count below goal", func(t *testing.T) {
		session := newTestSession(100, StatusWaiting)
		participant := addTestParticipant(t, session, "alice")
		session.Status = StatusActive

		view, ended := session.UpdateWordCount(participant.ID, 50)

		if ended {
			t.Error("session should not have ended")
		}
		if view.WordCount != 50 {
			t.Errorf("WordCount = %d, want 50", view.WordCount)
		}
		if view.Completed {
			t.Error("participant should not be completed yet")
		}
		if view.FinishOrder != 0 {
			t.Errorf("FinishOrder = %d, want 0", view.FinishOrder)
		}
	})

	t.Run("marks completed when word count reaches goal", func(t *testing.T) {
		session := newTestSession(100, StatusWaiting)
		participant := addTestParticipant(t, session, "alice")
		addTestParticipant(t, session, "bob") // bob keeps session alive
		session.Status = StatusActive

		view, ended := session.UpdateWordCount(participant.ID, 100)

		if ended {
			t.Error("session should not have ended while bob has not completed")
		}
		if !view.Completed {
			t.Error("participant should be completed")
		}
		if view.FinishOrder != 1 {
			t.Errorf("FinishOrder = %d, want 1", view.FinishOrder)
		}
	})

	t.Run("completion is not applied twice", func(t *testing.T) {
		session := newTestSession(100, StatusWaiting)
		participant := addTestParticipant(t, session, "alice")
		addTestParticipant(t, session, "bob")
		session.Status = StatusActive

		session.UpdateWordCount(participant.ID, 100)             // first completion
		view, _ := session.UpdateWordCount(participant.ID, 150)  // second update

		if view.FinishOrder != 1 {
			t.Errorf("FinishOrder = %d after second update, want 1", view.FinishOrder)
		}
		if session.finishCount != 1 {
			t.Errorf("finishCount = %d, want 1", session.finishCount)
		}
	})

	t.Run("all participants done ends session", func(t *testing.T) {
		session := newTestSession(10, StatusWaiting)
		alice := addTestParticipant(t, session, "alice")
		bob := addTestParticipant(t, session, "bob")
		session.Status = StatusActive

		_, ended := session.UpdateWordCount(alice.ID, 10)
		if ended {
			t.Error("session should not have ended yet — bob hasn't finished")
		}

		_, ended = session.UpdateWordCount(bob.ID, 10)
		if !ended {
			t.Error("session should have ended after all participants completed")
		}
		if session.Status != StatusEnded {
			t.Errorf("Status = %q, want %q", session.Status, StatusEnded)
		}
		select {
		case <-session.doneCh:
			// expected
		default:
			t.Error("doneCh should be closed")
		}
	})

	t.Run("no-op when session is not active", func(t *testing.T) {
		session := newTestSession(100, StatusWaiting)
		participant := addTestParticipant(t, session, "alice")

		view, ended := session.UpdateWordCount(participant.ID, 100)

		if ended {
			t.Error("should not end a non-active session")
		}
		if view.ID != "" {
			t.Error("expected empty view for non-active session")
		}
	})

	t.Run("no-op for unknown participant", func(t *testing.T) {
		session := newTestSession(100, StatusActive)

		view, ended := session.UpdateWordCount("nonexistent", 100)

		if ended {
			t.Error("should not end session on unknown participant")
		}
		if view.ID != "" {
			t.Error("expected empty view for unknown participant")
		}
	})
}

func TestFinishOrder(t *testing.T) {
	session := newTestSession(10, StatusWaiting)
	alice := addTestParticipant(t, session, "alice")
	bob := addTestParticipant(t, session, "bob")
	carol := addTestParticipant(t, session, "carol")
	session.Status = StatusActive

	viewA, _ := session.UpdateWordCount(alice.ID, 10)
	viewB, _ := session.UpdateWordCount(bob.ID, 10)
	viewC, _ := session.UpdateWordCount(carol.ID, 10)

	if viewA.FinishOrder != 1 {
		t.Errorf("alice FinishOrder = %d, want 1", viewA.FinishOrder)
	}
	if viewB.FinishOrder != 2 {
		t.Errorf("bob FinishOrder = %d, want 2", viewB.FinishOrder)
	}
	if viewC.FinishOrder != 3 {
		t.Errorf("carol FinishOrder = %d, want 3", viewC.FinishOrder)
	}
}

func TestView(t *testing.T) {
	session := newTestSession(100, StatusWaiting)
	addTestParticipant(t, session, "alice")
	addTestParticipant(t, session, "bob")

	view := session.View()

	if view.ID != session.ID {
		t.Errorf("ID = %q, want %q", view.ID, session.ID)
	}
	if view.Goal != 100 {
		t.Errorf("Goal = %d, want 100", view.Goal)
	}
	if view.Status != StatusWaiting {
		t.Errorf("Status = %q, want %q", view.Status, StatusWaiting)
	}
	if len(view.Participants) != 2 {
		t.Errorf("len(Participants) = %d, want 2", len(view.Participants))
	}
}

func TestBroadcast(t *testing.T) {
	session := newTestSession(100, StatusWaiting)
	ch1 := makeSendCh()
	ch2 := makeSendCh()

	participant1, _ := session.AddParticipant("alice", ch1)
	participant2, _ := session.AddParticipant("bob", ch2)
	_ = participant1
	_ = participant2

	msg := []byte(`{"type":"test"}`)
	session.Broadcast(msg)

	assertReceived(t, ch1, msg, "alice")
	assertReceived(t, ch2, msg, "bob")
}

func TestBroadcastExcept(t *testing.T) {
	session := newTestSession(100, StatusWaiting)
	ch1 := makeSendCh()
	ch2 := makeSendCh()

	participant1, _ := session.AddParticipant("alice", ch1)
	_, _ = session.AddParticipant("bob", ch2)

	msg := []byte(`{"type":"test"}`)
	session.BroadcastExcept(msg, participant1.ID)

	assertNotReceived(t, ch1, "alice (excluded)")
	assertReceived(t, ch2, msg, "bob")
}

func TestSendTo(t *testing.T) {
	session := newTestSession(100, StatusWaiting)
	ch1 := makeSendCh()
	ch2 := makeSendCh()

	participant1, _ := session.AddParticipant("alice", ch1)
	_, _ = session.AddParticipant("bob", ch2)

	msg := []byte(`{"type":"test"}`)
	session.SendTo(participant1.ID, msg)

	assertReceived(t, ch1, msg, "alice")
	assertNotReceived(t, ch2, "bob (not targeted)")
}

func assertReceived(t *testing.T, ch chan []byte, want []byte, label string) {
	t.Helper()
	select {
	case got := <-ch:
		if string(got) != string(want) {
			t.Errorf("%s: received %q, want %q", label, got, want)
		}
	default:
		t.Errorf("%s: expected message but channel was empty", label)
	}
}

func assertNotReceived(t *testing.T, ch chan []byte, label string) {
	t.Helper()
	select {
	case msg := <-ch:
		t.Errorf("%s: received unexpected message %q", label, msg)
	default:
		// expected
	}
}

func TestEndIfHostLeft(t *testing.T) {
	t.Run("ends session and returns true when host disconnects from waiting session", func(t *testing.T) {
		session := newTestSession(100, StatusWaiting)
		host := addTestParticipant(t, session, "alice")
		guestCh := makeSendCh()
		addTestParticipant(t, session, "bob")
		// Manually set guest sendCh so broadcast can reach bob.
		session.mu.Lock()
		for _, participant := range session.participants {
			if participant.Username == "bob" {
				participant.sendCh = guestCh
			}
		}
		session.mu.Unlock()

		ended := session.EndIfHostLeft(host)

		if !ended {
			t.Error("EndIfHostLeft should return true for host in waiting session")
		}
		if session.Status != StatusEnded {
			t.Errorf("Status = %q, want %q", session.Status, StatusEnded)
		}
		select {
		case <-session.doneCh:
			// expected
		default:
			t.Error("doneCh should be closed after end")
		}

		// Simulate the broadcast that ws.go performs after EndIfHostLeft returns true.
		session.Broadcast([]byte(`{"type":"session_ended"}`))
		select {
		case msg := <-guestCh:
			var parsed map[string]any
			if err := json.Unmarshal(msg, &parsed); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if parsed["type"] != "session_ended" {
				t.Errorf("type = %q, want session_ended", parsed["type"])
			}
		default:
			t.Error("guest should have received session_ended")
		}
	})

	t.Run("returns false and does not end session for non-host participant", func(t *testing.T) {
		session := newTestSession(100, StatusWaiting)
		addTestParticipant(t, session, "alice") // host
		guest := addTestParticipant(t, session, "bob")

		ended := session.EndIfHostLeft(guest)

		if ended {
			t.Error("EndIfHostLeft should return false for non-host participant")
		}
		if session.Status != StatusWaiting {
			t.Errorf("Status = %q, want waiting", session.Status)
		}
	})

	t.Run("returns false when session is already active", func(t *testing.T) {
		session := newTestSession(100, StatusWaiting)
		host := addTestParticipant(t, session, "alice")
		session.Start()

		ended := session.EndIfHostLeft(host)

		if ended {
			t.Error("EndIfHostLeft should return false when session is active")
		}
	})
}
