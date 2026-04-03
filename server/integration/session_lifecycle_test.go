//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestSoloSessionCreateStartComplete tests that a single user can create a
// session, connect via WebSocket, start the session, send word count updates,
// and have the session end automatically once the goal is reached.
func TestSoloSessionCreateStartComplete(t *testing.T) {
	t.Parallel()

	server, baseURL := startTestServer()
	defer server.Close()

	sessionID, participantID, err := createSession(baseURL, 10, "solo", true)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	// The host participant was already registered by POST /api/sessions, so
	// the first WebSocket message must be "rejoin" (not "join").
	client := newTestClient(t)
	if err := client.dial(baseURL, sessionID); err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.close()

	if err := client.sendJSON(map[string]any{"type": "rejoin", "participantId": participantID}); err != nil {
		t.Fatalf("sendJSON rejoin: %v", err)
	}

	var stateMessage struct {
		Type    string `json:"type"`
		Session struct {
			Status string `json:"status"`
			Goal   int    `json:"goal"`
			// Participants is a slice but we only need the count.
			Participants []json.RawMessage `json:"participants"`
		} `json:"session"`
	}
	if err := client.readJSON(&stateMessage); err != nil {
		t.Fatalf("readJSON session_state: %v", err)
	}
	if stateMessage.Type != "session_state" {
		t.Fatalf("expected session_state, got %q", stateMessage.Type)
	}
	if stateMessage.Session.Status != "waiting" {
		t.Errorf("status: got %q, want %q", stateMessage.Session.Status, "waiting")
	}
	if stateMessage.Session.Goal != 10 {
		t.Errorf("goal: got %d, want 10", stateMessage.Session.Goal)
	}
	if len(stateMessage.Session.Participants) != 1 {
		t.Errorf("participants: got %d, want 1", len(stateMessage.Session.Participants))
	}

	if err := client.sendJSON(map[string]any{"type": "start"}); err != nil {
		t.Fatalf("sendJSON start: %v", err)
	}
	if _, err := client.readUntil("session_started"); err != nil {
		t.Fatalf("readUntil session_started: %v", err)
	}

	// Send WordCount=5 — expect partial progress.
	if err := client.sendJSON(map[string]any{"type": "update", "wordCount": 5}); err != nil {
		t.Fatalf("sendJSON update 5: %v", err)
	}
	raw, err := client.readUntil("participant_updated")
	if err != nil {
		t.Fatalf("readUntil participant_updated (5): %v", err)
	}
	var updateMessage struct {
		Participant participantMsg `json:"participant"`
	}
	if err := json.Unmarshal(raw, &updateMessage); err != nil {
		t.Fatalf("unmarshal participant_updated (5): %v", err)
	}
	if updateMessage.Participant.WordCount != 5 {
		t.Errorf("WordCount: got %d, want 5", updateMessage.Participant.WordCount)
	}
	if updateMessage.Participant.Completed {
		t.Error("Completed: got true, want false after WordCount=5")
	}

	// Send WordCount=10 — expect completion.
	if err := client.sendJSON(map[string]any{"type": "update", "wordCount": 10}); err != nil {
		t.Fatalf("sendJSON update 10: %v", err)
	}
	raw, err = client.readUntil("participant_updated")
	if err != nil {
		t.Fatalf("readUntil participant_updated (10): %v", err)
	}
	if err := json.Unmarshal(raw, &updateMessage); err != nil {
		t.Fatalf("unmarshal participant_updated (10): %v", err)
	}
	if updateMessage.Participant.WordCount != 10 {
		t.Errorf("WordCount: got %d, want 10", updateMessage.Participant.WordCount)
	}
	if !updateMessage.Participant.Completed {
		t.Error("Completed: got false, want true after WordCount=10")
	}
	if updateMessage.Participant.FinishOrder != 1 {
		t.Errorf("FinishOrder: got %d, want 1", updateMessage.Participant.FinishOrder)
	}

	// The session should auto-end once all participants complete.
	if _, err := client.readUntil("session_ended"); err != nil {
		t.Fatalf("readUntil session_ended: %v", err)
	}
}

// TestMultiUserSessionJoinAndComplete tests that multiple users can join,
// the host starts the session, all users reach the goal, and the session ends
// automatically with correct finish ordering.
func TestMultiUserSessionJoinAndComplete(t *testing.T) {
	t.Parallel()

	server, baseURL := startTestServer()
	defer server.Close()

	sessionID, hostParticipantID, err := createSession(baseURL, 5, "host", true)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	host := newTestClient(t)
	if err := host.dial(baseURL, sessionID); err != nil {
		t.Fatalf("host dial: %v", err)
	}
	defer host.close()

	if err := host.sendJSON(map[string]any{"type": "rejoin", "participantId": hostParticipantID}); err != nil {
		t.Fatalf("host sendJSON rejoin: %v", err)
	}
	if _, err := host.readUntil("session_state"); err != nil {
		t.Fatalf("host readUntil session_state: %v", err)
	}

	userB := newTestClient(t)
	if err := userB.dial(baseURL, sessionID); err != nil {
		t.Fatalf("userB dial: %v", err)
	}
	defer userB.close()

	if err := userB.sendJSON(map[string]any{"type": "join", "username": "userB"}); err != nil {
		t.Fatalf("userB sendJSON join: %v", err)
	}

	var userBStateMessage struct {
		Type            string `json:"type"`
		MyParticipantID string `json:"myParticipantId"`
	}
	if err := userB.readJSON(&userBStateMessage); err != nil {
		t.Fatalf("userB readJSON session_state: %v", err)
	}
	if userBStateMessage.Type != "session_state" {
		t.Fatalf("userB: expected session_state, got %q", userBStateMessage.Type)
	}
	userBParticipantID := userBStateMessage.MyParticipantID

	raw, err := host.readUntil("participant_joined")
	if err != nil {
		t.Fatalf("host readUntil participant_joined: %v", err)
	}
	var joinedMessage struct {
		Participant participantMsg `json:"participant"`
	}
	if err := json.Unmarshal(raw, &joinedMessage); err != nil {
		t.Fatalf("unmarshal participant_joined: %v", err)
	}
	if joinedMessage.Participant.Username != "userB" {
		t.Errorf("participant_joined username: got %q, want %q", joinedMessage.Participant.Username, "userB")
	}

	if err := host.sendJSON(map[string]any{"type": "start"}); err != nil {
		t.Fatalf("host sendJSON start: %v", err)
	}
	if _, err := host.readUntil("session_started"); err != nil {
		t.Fatalf("host readUntil session_started: %v", err)
	}
	if _, err := userB.readUntil("session_started"); err != nil {
		t.Fatalf("userB readUntil session_started: %v", err)
	}

	// User B completes first.
	if err := userB.sendJSON(map[string]any{"type": "update", "wordCount": 5}); err != nil {
		t.Fatalf("userB sendJSON update 5: %v", err)
	}
	assertParticipantUpdated(t, "host (userB update)", host, userBParticipantID, 1)
	assertParticipantUpdated(t, "userB (userB update)", userB, userBParticipantID, 1)

	// Host completes second.
	if err := host.sendJSON(map[string]any{"type": "update", "wordCount": 5}); err != nil {
		t.Fatalf("host sendJSON update 5: %v", err)
	}
	assertParticipantUpdated(t, "host (host update)", host, hostParticipantID, 2)
	assertParticipantUpdated(t, "userB (host update)", userB, hostParticipantID, 2)

	if _, err := host.readUntil("session_ended"); err != nil {
		t.Fatalf("host readUntil session_ended: %v", err)
	}
	if _, err := userB.readUntil("session_ended"); err != nil {
		t.Fatalf("userB readUntil session_ended: %v", err)
	}
}

// TestSoloSessionHostManuallyEnds tests that the host can end an active
// session at any time by sending the "end" message.
func TestSoloSessionHostManuallyEnds(t *testing.T) {
	t.Parallel()

	server, baseURL := startTestServer()
	defer server.Close()

	client, _ := startSoloSession(t, baseURL, 100)
	defer client.close()

	if err := client.sendJSON(map[string]any{"type": "end"}); err != nil {
		t.Fatalf("sendJSON end: %v", err)
	}
	if _, err := client.readUntil("session_ended"); err != nil {
		t.Fatalf("readUntil session_ended: %v", err)
	}
}

// TestHostLeavesWaitingSession verifies that when the host disconnects while
// the session is still waiting, the session ends, remaining participants
// receive session_ended, and a subsequent join attempt is rejected.
func TestHostLeavesWaitingSession(t *testing.T) {
	t.Parallel()

	server, baseURL := startTestServer()
	defer server.Close()

	sessionID, hostID, err := createSession(baseURL, 10, "host", true)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	host := newTestClient(t)
	if err := host.dial(baseURL, sessionID); err != nil {
		t.Fatalf("host dial: %v", err)
	}
	if err := host.sendJSON(map[string]any{"type": "rejoin", "participantId": hostID}); err != nil {
		host.close()
		t.Fatalf("host rejoin: %v", err)
	}
	if _, err := host.readUntil("session_state"); err != nil {
		host.close()
		t.Fatalf("host session_state: %v", err)
	}

	userB := newTestClient(t)
	if err := userB.dial(baseURL, sessionID); err != nil {
		host.close()
		t.Fatalf("userB dial: %v", err)
	}
	defer userB.close()
	if err := userB.sendJSON(map[string]any{"type": "join", "username": "userB"}); err != nil {
		host.close()
		t.Fatalf("userB join: %v", err)
	}
	if _, err := userB.readUntil("session_state"); err != nil {
		host.close()
		t.Fatalf("userB session_state: %v", err)
	}
	if _, err := host.readUntil("participant_joined"); err != nil {
		host.close()
		t.Fatalf("host participant_joined: %v", err)
	}

	// Host disconnects while session is still waiting.
	host.close()

	// User B should receive session_ended.
	if _, err := userB.readUntil("session_ended"); err != nil {
		t.Fatalf("userB session_ended: %v", err)
	}

	// A new user attempts to join the now-ended session and should receive an error.
	newUser := newTestClient(t)
	if err := newUser.dial(baseURL, sessionID); err != nil {
		t.Fatalf("newUser dial: %v", err)
	}
	defer newUser.close()
	if err := newUser.sendJSON(map[string]any{"type": "join", "username": "newUser"}); err != nil {
		t.Fatalf("newUser join: %v", err)
	}
	raw, err := newUser.readUntil("error")
	if err != nil {
		t.Fatalf("newUser readUntil error: %v", err)
	}
	var errorMessage struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &errorMessage); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if errorMessage.Message == "" {
		t.Error("expected non-empty error message for joining ended session")
	}
}

// TestInvalidSessionCreationBadGoal verifies that POST /api/sessions with
// goal < 1 returns 400 with a meaningful error message.
func TestInvalidSessionCreationBadGoal(t *testing.T) {
	t.Parallel()

	server, baseURL := startTestServer()
	defer server.Close()

	cases := []struct {
		name string
		goal int
	}{
		{"zero goal", 0},
		{"negative goal", -5},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := strings.NewReader(fmt.Sprintf(`{"goal":%d,"username":"user","public":true}`, tc.goal))
			response, err := http.Post(baseURL+"/api/sessions", "application/json", body)
			if err != nil {
				t.Fatalf("POST /api/sessions: %v", err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusBadRequest {
				t.Errorf("status: got %d, want %d", response.StatusCode, http.StatusBadRequest)
			}
			responseBody, _ := io.ReadAll(response.Body)
			if len(strings.TrimSpace(string(responseBody))) == 0 {
				t.Error("expected non-empty error body")
			}
		})
	}
}
