//go:build integration

package integration

import (
	"testing"

	"github.com/todd-beckman/writerace/server/internal/session"
)

// TestPublicWaitingSessionAppearsInLobby verifies that a newly created public
// session in waiting status appears in GET /api/sessions with the correct goal,
// host username, and writer count.
func TestPublicWaitingSessionAppearsInLobby(t *testing.T) {
	t.Parallel()

	server, baseURL := startTestServer()
	defer server.Close()

	_, _, err := createSession(baseURL, 42, "alice", true)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	sessions, err := listSessions(baseURL)
	if err != nil {
		t.Fatalf("listSessions: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("session count: got %d, want 1", len(sessions))
	}
	listedSession := sessions[0]
	if listedSession.Goal != 42 {
		t.Errorf("goal: got %d, want 42", listedSession.Goal)
	}
	if listedSession.HostUsername != "alice" {
		t.Errorf("hostUsername: got %q, want %q", listedSession.HostUsername, "alice")
	}
	if listedSession.WriterCount != 1 {
		t.Errorf("writerCount: got %d, want 1", listedSession.WriterCount)
	}
}

// TestExcludedSessionsNotInLobby verifies that private, active, and full
// sessions do not appear in the GET /api/sessions lobby listing.
func TestExcludedSessionsNotInLobby(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		setup func(t *testing.T, baseURL string)
	}{
		{
			name: "private session",
			setup: func(t *testing.T, baseURL string) {
				t.Helper()
				if _, _, err := createSession(baseURL, 10, "host", false); err != nil {
					t.Fatalf("createSession: %v", err)
				}
			},
		},
		{
			name: "active session",
			setup: func(t *testing.T, baseURL string) {
				t.Helper()
				client, _ := startSoloSession(t, baseURL, 100)
				// startSoloSession already started the session; leave it running.
				t.Cleanup(client.close)
			},
		},
		{
			name: "full session",
			setup: func(t *testing.T, baseURL string) {
				t.Helper()
				sessionID, hostID, err := createSession(baseURL, 10, "host", true)
				if err != nil {
					t.Fatalf("createSession: %v", err)
				}
				host := newTestClient(t)
				if err := host.dial(baseURL, sessionID); err != nil {
					t.Fatalf("host dial: %v", err)
				}
				t.Cleanup(host.close)
				if err := host.sendJSON(map[string]any{"type": "rejoin", "participantId": hostID}); err != nil {
					t.Fatalf("host rejoin: %v", err)
				}
				if _, err := host.readUntil("session_state"); err != nil {
					t.Fatalf("host session_state: %v", err)
				}
				for i := 0; i < session.MaxParticipants-1; i++ {
					client := newTestClient(t)
					if err := client.dial(baseURL, sessionID); err != nil {
						t.Fatalf("client %d dial: %v", i, err)
					}
					t.Cleanup(client.close)
					if err := client.sendJSON(map[string]any{"type": "join", "username": string(rune('a'+i)) + "user"}); err != nil {
						t.Fatalf("client %d join: %v", i, err)
					}
					if _, err := client.readUntil("session_state"); err != nil {
						t.Fatalf("client %d session_state: %v", i, err)
					}
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, baseURL := startTestServer()
			defer srv.Close()

			tc.setup(t, baseURL)

			sessions, err := listSessions(baseURL)
			if err != nil {
				t.Fatalf("listSessions: %v", err)
			}
			if len(sessions) != 0 {
				t.Errorf("expected 0 sessions in lobby, got %d", len(sessions))
			}
		})
	}
}
