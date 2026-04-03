//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/todd-beckman/writerace/server/internal/handler"
	"github.com/todd-beckman/writerace/server/internal/session"
)

// wsReadTimeout is the maximum time a single read (or a readUntil loop) will
// wait for a message before failing the test. Without this, a missing broadcast
// causes the test to hang indefinitely.
const wsReadTimeout = 2 * time.Second

// lobbyView is used to decode GET /api/sessions responses in tests.
type lobbyView struct {
	ID           string `json:"id"`
	Goal         int    `json:"goal"`
	HostUsername string `json:"hostUsername"`
	WriterCount  int    `json:"writerCount"`
}

// participantMsg is used to decode participant fields in WebSocket messages.
type participantMsg struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	WordCount   int    `json:"wordCount"`
	Connected   bool   `json:"connected"`
	Completed   bool   `json:"completed"`
	JoinOrder   int    `json:"joinOrder"`
	FinishOrder int    `json:"finishOrder"`
}

// startTestServer creates a session manager, registers all routes, and starts
// an httptest.Server. Returns the running server and its base URL.
// The caller is responsible for calling server.Close() when done.
func startTestServer() (*httptest.Server, string) {
	manager := session.NewManager()
	server := httptest.NewUnstartedServer(nil)
	allowedOrigin := "http://" + server.Listener.Addr().String()
	server.Config.Handler = handler.Cors(allowedOrigin, handler.NewMux(allowedOrigin, manager, nil))
	server.Start()
	return server, server.URL
}

// testClient wraps a *websocket.Conn with test-friendly helpers that enforce
// read timeouts so a missing message fails fast rather than hanging.
type testClient struct {
	t    *testing.T
	conn *websocket.Conn
}

func newTestClient(t *testing.T) *testClient {
	t.Helper()
	return &testClient{t: t}
}

// dial connects to ws://<host>/ws/<sessionID>.
func (c *testClient) dial(serverURL, sessionID string) error {
	wsURL := strings.Replace(serverURL, "http://", "ws://", 1) + "/ws/" + sessionID
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial %s: %w", wsURL, err)
	}
	c.conn = conn
	return nil
}

// sendJSON marshals msg and sends it as a text WebSocket message.
func (c *testClient) sendJSON(msg any) error {
	return c.conn.WriteJSON(msg)
}

// sendRaw sends raw bytes as a text WebSocket message.
func (c *testClient) sendRaw(payload []byte) error {
	return c.conn.WriteMessage(websocket.TextMessage, payload)
}

// readJSON reads the next message (with a timeout) and unmarshals it into target.
func (c *testClient) readJSON(target any) error {
	c.conn.SetReadDeadline(time.Now().Add(wsReadTimeout))
	_, raw, err := c.conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("readJSON: %w", err)
	}
	return json.Unmarshal(raw, target)
}

// readUntil reads messages until one with the given type field arrives, then
// returns the full raw JSON for that message. The entire loop shares one
// wsReadTimeout budget, so a missing broadcast fails quickly.
func (c *testClient) readUntil(msgType string) (json.RawMessage, error) {
	c.conn.SetReadDeadline(time.Now().Add(wsReadTimeout))
	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("readUntil %q: %w", msgType, err)
		}
		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			continue
		}
		if envelope.Type == msgType {
			return json.RawMessage(raw), nil
		}
	}
}

// assertNoMessage asserts that no message arrives within timeout. A deadline
// error is the expected (happy) path; receiving any message fails the test.
func (c *testClient) assertNoMessage(t *testing.T, timeout time.Duration) {
	t.Helper()
	c.conn.SetReadDeadline(time.Now().Add(timeout))
	defer c.conn.SetReadDeadline(time.Time{})
	_, _, err := c.conn.ReadMessage()
	if err == nil {
		t.Error("assertNoMessage: received an unexpected message")
	}
}

// close sends a WebSocket close frame and closes the underlying connection.
func (c *testClient) close() {
	if c.conn == nil {
		return
	}
	c.conn.WriteMessage( //nolint
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
	)
	c.conn.Close()
}

// createSession calls POST /api/sessions and returns the new session ID and
// host participant ID.
func createSession(baseURL string, goal int, username string, public bool) (string, string, error) {
	body, err := json.Marshal(map[string]any{
		"goal":     goal,
		"username": username,
		"public":   public,
	})
	if err != nil {
		return "", "", err
	}
	response, err := http.Post(baseURL+"/api/sessions", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("createSession: unexpected status %d", response.StatusCode)
	}
	var result struct {
		ID            string `json:"id"`
		ParticipantID string `json:"participantId"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", "", err
	}
	return result.ID, result.ParticipantID, nil
}

// listSessions calls GET /api/sessions and returns the lobby listing.
func listSessions(baseURL string) ([]lobbyView, error) {
	response, err := http.Get(baseURL + "/api/sessions")
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var result []lobbyView
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// startSoloSession creates a session, connects the host via WebSocket, and
// starts the session. Returns the connected client and session ID. The caller
// is responsible for calling client.close().
func startSoloSession(t *testing.T, baseURL string, goal int) (*testClient, string) {
	t.Helper()

	sessionID, participantID, err := createSession(baseURL, goal, "solo", true)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	client := newTestClient(t)
	if err := client.dial(baseURL, sessionID); err != nil {
		t.Fatalf("dial: %v", err)
	}

	if err := client.sendJSON(map[string]any{"type": "rejoin", "participantId": participantID}); err != nil {
		client.close()
		t.Fatalf("sendJSON rejoin: %v", err)
	}
	if _, err := client.readUntil("session_state"); err != nil {
		client.close()
		t.Fatalf("readUntil session_state: %v", err)
	}

	if err := client.sendJSON(map[string]any{"type": "start"}); err != nil {
		client.close()
		t.Fatalf("sendJSON start: %v", err)
	}
	if _, err := client.readUntil("session_started"); err != nil {
		client.close()
		t.Fatalf("readUntil session_started: %v", err)
	}

	return client, sessionID
}

// assertParticipantUpdated reads the next "participant_updated" message from
// client and asserts that it describes a completed participant with the given
// ID and finish order. who is a label used in failure messages.
func assertParticipantUpdated(t *testing.T, who string, client *testClient, wantID string, wantFinishOrder int) {
	t.Helper()
	raw, err := client.readUntil("participant_updated")
	if err != nil {
		t.Fatalf("%s readUntil participant_updated: %v", who, err)
	}
	var msg struct {
		Participant participantMsg `json:"participant"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("%s unmarshal participant_updated: %v", who, err)
	}
	if msg.Participant.ID != wantID {
		t.Errorf("%s participant ID: got %q, want %q", who, msg.Participant.ID, wantID)
	}
	if !msg.Participant.Completed {
		t.Errorf("%s Completed: got false, want true", who)
	}
	if msg.Participant.FinishOrder != wantFinishOrder {
		t.Errorf("%s FinishOrder: got %d, want %d", who, msg.Participant.FinishOrder, wantFinishOrder)
	}
}

// twoUserSession holds the clients and IDs returned by startTwoUserSession.
type twoUserSession struct {
	host      *testClient
	hostID    string
	userB     *testClient
	userBID   string
	sessionID string
}

// startTwoUserSession creates a session, connects both host and a second user,
// starts the session, and returns the two clients with their participant IDs.
// The caller is responsible for calling host.close() and userB.close().
func startTwoUserSession(t *testing.T, baseURL string, goal int) twoUserSession {
	t.Helper()

	sessionID, hostID, err := createSession(baseURL, goal, "host", true)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	host := newTestClient(t)
	if err := host.dial(baseURL, sessionID); err != nil {
		t.Fatalf("host dial: %v", err)
	}
	if err := host.sendJSON(map[string]any{"type": "rejoin", "participantId": hostID}); err != nil {
		host.close()
		t.Fatalf("host sendJSON rejoin: %v", err)
	}
	if _, err := host.readUntil("session_state"); err != nil {
		host.close()
		t.Fatalf("host readUntil session_state: %v", err)
	}

	userB := newTestClient(t)
	if err := userB.dial(baseURL, sessionID); err != nil {
		host.close()
		t.Fatalf("userB dial: %v", err)
	}
	if err := userB.sendJSON(map[string]any{"type": "join", "username": "userB"}); err != nil {
		host.close()
		userB.close()
		t.Fatalf("userB sendJSON join: %v", err)
	}
	var userBState struct {
		MyParticipantID string `json:"myParticipantId"`
	}
	if err := userB.readJSON(&userBState); err != nil {
		host.close()
		userB.close()
		t.Fatalf("userB readJSON session_state: %v", err)
	}
	// Drain participant_joined from host.
	if _, err := host.readUntil("participant_joined"); err != nil {
		host.close()
		userB.close()
		t.Fatalf("host readUntil participant_joined: %v", err)
	}

	if err := host.sendJSON(map[string]any{"type": "start"}); err != nil {
		host.close()
		userB.close()
		t.Fatalf("host sendJSON start: %v", err)
	}
	if _, err := host.readUntil("session_started"); err != nil {
		host.close()
		userB.close()
		t.Fatalf("host readUntil session_started: %v", err)
	}
	if _, err := userB.readUntil("session_started"); err != nil {
		host.close()
		userB.close()
		t.Fatalf("userB readUntil session_started: %v", err)
	}

	return twoUserSession{
		host:      host,
		hostID:    hostID,
		userB:     userB,
		userBID:   userBState.MyParticipantID,
		sessionID: sessionID,
	}
}
