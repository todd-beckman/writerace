//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/todd-beckman/writerace/server/internal/session"
)

// TestWordCountProgressBroadcasting tests that when one user sends a word
// count update, all other connected users receive the broadcast with the
// correct data.
func TestWordCountProgressBroadcasting(t *testing.T) {
	t.Parallel()

	server, baseURL := startTestServer()
	defer server.Close()

	ts := startTwoUserSession(t, baseURL, 100)
	defer ts.host.close()
	defer ts.userB.close()

	if err := ts.host.sendJSON(map[string]any{"type": "update", "wordCount": 3}); err != nil {
		t.Fatalf("host sendJSON update: %v", err)
	}

	raw, err := ts.userB.readUntil("participant_updated")
	if err != nil {
		t.Fatalf("userB readUntil participant_updated: %v", err)
	}
	var updateMessage struct {
		Participant participantMsg `json:"participant"`
	}
	if err := json.Unmarshal(raw, &updateMessage); err != nil {
		t.Fatalf("unmarshal participant_updated: %v", err)
	}
	if updateMessage.Participant.ID != ts.hostID {
		t.Errorf("participant ID: got %q, want %q", updateMessage.Participant.ID, ts.hostID)
	}
	if updateMessage.Participant.WordCount != 3 {
		t.Errorf("WordCount: got %d, want 3", updateMessage.Participant.WordCount)
	}
}

// TestRejoinAfterDisconnect tests that a user who disconnects from an active
// session can rejoin using their participant ID and resume where they left off.
func TestRejoinAfterDisconnect(t *testing.T) {
	t.Parallel()

	server, baseURL := startTestServer()
	defer server.Close()

	ts := startTwoUserSession(t, baseURL, 100)
	defer ts.host.close()

	// User B sends update with WordCount=3.
	if err := ts.userB.sendJSON(map[string]any{"type": "update", "wordCount": 3}); err != nil {
		t.Fatalf("userB sendJSON update: %v", err)
	}
	// Drain the word count broadcast from the host before disconnecting, so the
	// next participant_updated the host reads is the disconnect notification.
	if _, err := ts.host.readUntil("participant_updated"); err != nil {
		t.Fatalf("host readUntil participant_updated (word count): %v", err)
	}

	// User B disconnects.
	ts.userB.close()

	// Host reads participant_updated showing User B with Connected=false.
	raw, err := ts.host.readUntil("participant_updated")
	if err != nil {
		t.Fatalf("host readUntil participant_updated (disconnect): %v", err)
	}
	var updateMessage struct {
		Participant participantMsg `json:"participant"`
	}
	if err := json.Unmarshal(raw, &updateMessage); err != nil {
		t.Fatalf("unmarshal participant_updated (disconnect): %v", err)
	}
	if updateMessage.Participant.ID != ts.userBID {
		t.Errorf("disconnect: participant ID: got %q, want %q", updateMessage.Participant.ID, ts.userBID)
	}
	if updateMessage.Participant.Connected {
		t.Error("disconnect: Connected: got true, want false")
	}

	// User B rejoins on a new connection.
	newUserB := newTestClient(t)
	if err := newUserB.dial(baseURL, ts.sessionID); err != nil {
		t.Fatalf("userB redial: %v", err)
	}
	defer newUserB.close()

	if err := newUserB.sendJSON(map[string]any{"type": "rejoin", "participantId": ts.userBID}); err != nil {
		t.Fatalf("userB sendJSON rejoin: %v", err)
	}

	// User B reads session_state: WordCount=3 and Connected=true.
	var stateMessage struct {
		Type    string `json:"type"`
		Session struct {
			Participants []participantMsg `json:"participants"`
		} `json:"session"`
	}
	if err := newUserB.readJSON(&stateMessage); err != nil {
		t.Fatalf("userB readJSON session_state: %v", err)
	}
	if stateMessage.Type != "session_state" {
		t.Fatalf("expected session_state, got %q", stateMessage.Type)
	}
	var userBView *participantMsg
	for i := range stateMessage.Session.Participants {
		if stateMessage.Session.Participants[i].ID == ts.userBID {
			userBView = &stateMessage.Session.Participants[i]
			break
		}
	}
	if userBView == nil {
		t.Fatal("userB participant not found in session_state")
	}
	if userBView.WordCount != 3 {
		t.Errorf("WordCount after rejoin: got %d, want 3", userBView.WordCount)
	}
	if !userBView.Connected {
		t.Error("Connected after rejoin: got false, want true")
	}

	// Host reads participant_updated showing User B with Connected=true.
	raw, err = ts.host.readUntil("participant_updated")
	if err != nil {
		t.Fatalf("host readUntil participant_updated (reconnect): %v", err)
	}
	if err := json.Unmarshal(raw, &updateMessage); err != nil {
		t.Fatalf("unmarshal participant_updated (reconnect): %v", err)
	}
	if updateMessage.Participant.ID != ts.userBID {
		t.Errorf("reconnect: participant ID: got %q, want %q", updateMessage.Participant.ID, ts.userBID)
	}
	if !updateMessage.Participant.Connected {
		t.Error("reconnect: Connected: got false, want true")
	}
}

// TestStickyCompletionAfterWordCountDecrease tests that once a participant
// reaches the goal, a subsequent lower word count does not revoke their
// completion or change their finish order, and the session does not end until
// all other participants have also completed.
func TestStickyCompletionAfterWordCountDecrease(t *testing.T) {
	t.Parallel()

	server, baseURL := startTestServer()
	defer server.Close()

	ts := startTwoUserSession(t, baseURL, 10)
	defer ts.host.close()
	defer ts.userB.close()

	// Host reaches the goal first.
	if err := ts.host.sendJSON(map[string]any{"type": "update", "wordCount": 10}); err != nil {
		t.Fatalf("host sendJSON update 10: %v", err)
	}
	assertParticipantUpdated(t, "host (wordcount=10)", ts.host, ts.hostID, 1)
	assertParticipantUpdated(t, "userB (wordcount=10)", ts.userB, ts.hostID, 1)

	// Host sends a lower word count after completing.
	if err := ts.host.sendJSON(map[string]any{"type": "update", "wordCount": 5}); err != nil {
		t.Fatalf("host sendJSON update 5: %v", err)
	}
	// Both clients read participant_updated: Completed and FinishOrder unchanged.
	assertParticipantUpdated(t, "host (wordcount=5)", ts.host, ts.hostID, 1)
	assertParticipantUpdated(t, "userB (wordcount=5)", ts.userB, ts.hostID, 1)

	// Session must still be active — User B has not finished. Verify by
	// checking that no session_ended message has arrived.
	ts.host.assertNoMessage(t, 100*time.Millisecond)
	ts.userB.assertNoMessage(t, 100*time.Millisecond)
}

// TestIllegalUsernameRejected verifies that a join with a username containing
// illegal characters is rejected with an error and the connection is closed.
func TestIllegalUsernameRejected(t *testing.T) {
	t.Parallel()

	server, baseURL := startTestServer()
	defer server.Close()

	sessionID, _, err := createSession(baseURL, 10, "host", true)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	client := newTestClient(t)
	if err := client.dial(baseURL, sessionID); err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.close()

	if err := client.sendJSON(map[string]any{"type": "join", "username": "hack<script>"}); err != nil {
		t.Fatalf("sendJSON join: %v", err)
	}

	raw, err := client.readUntil("error")
	if err != nil {
		t.Fatalf("readUntil error: %v", err)
	}
	var errorMessage struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &errorMessage); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if errorMessage.Message == "" {
		t.Error("expected non-empty error message for illegal username")
	}

	// Server should close the connection after rejecting the join.
	var dummy struct{}
	if err := client.readJSON(&dummy); err == nil {
		t.Error("expected connection to be closed after illegal username, but read succeeded")
	}
}

// TestNegativeWordCountIgnored verifies that an update with a negative word
// count is silently dropped and does not broadcast to other users.
func TestNegativeWordCountIgnored(t *testing.T) {
	t.Parallel()

	server, baseURL := startTestServer()
	defer server.Close()

	ts := startTwoUserSession(t, baseURL, 100)
	defer ts.host.close()
	defer ts.userB.close()

	if err := ts.host.sendJSON(map[string]any{"type": "update", "wordCount": -1}); err != nil {
		t.Fatalf("sendJSON update -1: %v", err)
	}

	ts.userB.assertNoMessage(t, 200*time.Millisecond)
}

// TestInvalidPayloads verifies that the server gracefully handles malformed or
// unexpected WebSocket messages without crashing, and can still serve valid
// messages afterward.
func TestInvalidPayloads(t *testing.T) {
	t.Parallel()

	server, baseURL := startTestServer()
	defer server.Close()

	cases := []struct {
		name    string
		payload string
	}{
		{"invalid JSON", "not json at all"},
		{"unknown message type", `{"type":"unknown"}`},
		{"missing type field", `{"wordCount":5}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, _ := startSoloSession(t, baseURL, 100)
			defer client.close()

			if err := client.sendRaw([]byte(tc.payload)); err != nil {
				t.Fatalf("sendRaw: %v", err)
			}

			// The server must survive the invalid payload. A subsequent valid
			// update should be handled normally.
			if err := client.sendJSON(map[string]any{"type": "update", "wordCount": 1}); err != nil {
				t.Fatalf("sendJSON valid update: %v", err)
			}
			if _, err := client.readUntil("participant_updated"); err != nil {
				t.Fatalf("readUntil participant_updated after invalid payload: %v", err)
			}
		})
	}
}

// TestSessionAtMaxCapacity verifies that once a session reaches MaxParticipants,
// additional join attempts are rejected with an error message.
func TestSessionAtMaxCapacity(t *testing.T) {
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
	defer host.close()
	if err := host.sendJSON(map[string]any{"type": "rejoin", "participantId": hostID}); err != nil {
		t.Fatalf("host rejoin: %v", err)
	}
	if _, err := host.readUntil("session_state"); err != nil {
		t.Fatalf("host session_state: %v", err)
	}

	// Fill remaining slots.
	for i := 0; i < session.MaxParticipants-1; i++ {
		client := newTestClient(t)
		if err := client.dial(baseURL, sessionID); err != nil {
			t.Fatalf("client %d dial: %v", i, err)
		}
		defer client.close()
		if err := client.sendJSON(map[string]any{"type": "join", "username": fmt.Sprintf("user%d", i)}); err != nil {
			t.Fatalf("client %d join: %v", i, err)
		}
		if _, err := client.readUntil("session_state"); err != nil {
			t.Fatalf("client %d session_state: %v", i, err)
		}
	}

	// One more user attempts to join — should be rejected.
	extra := newTestClient(t)
	if err := extra.dial(baseURL, sessionID); err != nil {
		t.Fatalf("extra dial: %v", err)
	}
	defer extra.close()
	if err := extra.sendJSON(map[string]any{"type": "join", "username": "extra"}); err != nil {
		t.Fatalf("extra join: %v", err)
	}
	raw, err := extra.readUntil("error")
	if err != nil {
		t.Fatalf("extra readUntil error: %v", err)
	}
	var errorMessage struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &errorMessage); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if errorMessage.Message == "" {
		t.Error("expected non-empty error message for full session")
	}
}
