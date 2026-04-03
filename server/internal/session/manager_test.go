package session

import (
	"testing"
)

func TestSessionManagerCreateAndGet(t *testing.T) {
	manager := NewManager()

	newSession, hostID, err := manager.Create(200, "alice", false)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if newSession == nil {
		t.Fatal("session should not be nil")
	}
	if newSession.Goal != 200 {
		t.Errorf("Goal = %d, want 200", newSession.Goal)
	}
	if newSession.Status != StatusWaiting {
		t.Errorf("Status = %q, want %q", newSession.Status, StatusWaiting)
	}
	if hostID == "" {
		t.Error("hostID should not be empty")
	}
	if len(newSession.participants) != 1 {
		t.Errorf("participants count = %d, want 1", len(newSession.participants))
	}

	// host is stored with the returned hostID and is not yet connected
	host, ok := newSession.participants[hostID]
	if !ok {
		t.Fatalf("host participant %q not found in session", hostID)
	}
	if host.Username != "alice" {
		t.Errorf("host Username = %q, want %q", host.Username, "alice")
	}
	if host.JoinOrder != 1 {
		t.Errorf("host JoinOrder = %d, want 1", host.JoinOrder)
	}
	if host.Connected {
		t.Error("host Connected should be false until WebSocket opens")
	}

	got, found := manager.Get(newSession.ID)
	if !found {
		t.Fatal("Get should find the created session")
	}
	if got != newSession {
		t.Error("Get should return the same session pointer")
	}
}

func TestSessionManagerGetMissing(t *testing.T) {
	manager := NewManager()
	_, found := manager.Get("nonexistent")
	if found {
		t.Error("Get should return false for unknown session")
	}
}

func TestSessionManagerRemove(t *testing.T) {
	manager := NewManager()
	newSession, _, _ := manager.Create(100, "alice", false)

	manager.Remove(newSession.ID)

	_, found := manager.Get(newSession.ID)
	if found {
		t.Error("session should not be found after Remove")
	}
}

func TestPublicWaiting(t *testing.T) {
	t.Run("returns only public waiting non-full sessions", func(t *testing.T) {
		manager := NewManager()
		// Public waiting — should appear.
		manager.Create(500, "alice", true)
		// Private waiting — should not appear.
		manager.Create(500, "bob", false)
		// Public active — should not appear.
		startedSession, _, _ := manager.Create(500, "carol", true)
		startedSession.Start()
		// Public ended — should not appear.
		endedSession, _, _ := manager.Create(500, "dave", true)
		endedSession.End()

		entries := manager.PublicWaiting()
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}
		if entries[0].HostUsername != "alice" {
			t.Errorf("HostUsername = %q, want %q", entries[0].HostUsername, "alice")
		}
	})

	t.Run("excludes sessions at MaxParticipants capacity", func(t *testing.T) {
		manager := NewManager()
		newSession, _, _ := manager.Create(500, "host", true)
		for i := 1; i < MaxParticipants; i++ {
			newSession.AddParticipant(string(rune('a'+i)), makeSendCh())
		}

		entries := manager.PublicWaiting()
		if len(entries) != 0 {
			t.Errorf("expected 0 entries (at capacity), got %d", len(entries))
		}
	})

	t.Run("returns correct writer count and goal", func(t *testing.T) {
		manager := NewManager()
		newSession, _, _ := manager.Create(1000, "host", true)
		newSession.AddParticipant("writer1", makeSendCh())
		newSession.AddParticipant("writer2", makeSendCh())

		entries := manager.PublicWaiting()
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}
		if entries[0].WriterCount != 3 {
			t.Errorf("WriterCount = %d, want 3", entries[0].WriterCount)
		}
		if entries[0].Goal != 1000 {
			t.Errorf("Goal = %d, want 1000", entries[0].Goal)
		}
	})
}
