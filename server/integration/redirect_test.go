//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

// TestRedirectBroadcastsToConnectedClients verifies that
// POST /api/sessions/{id}/redirect sends a session_redirect message to all
// connected WebSocket clients with the provided nextSessionId.
func TestRedirectBroadcastsToConnectedClients(t *testing.T) {
	t.Parallel()

	server, baseURL := startTestServer()
	defer server.Close()

	ts := startTwoUserSession(t, baseURL, 100)
	defer ts.host.close()
	defer ts.userB.close()

	nextID := "next-session-abc"
	body, _ := json.Marshal(map[string]any{"nextSessionId": nextID})
	response, err := http.Post(baseURL+"/api/sessions/"+ts.sessionID+"/redirect", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST redirect: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("redirect status: got %d, want %d", response.StatusCode, http.StatusNoContent)
	}

	// Both clients should receive session_redirect with the correct nextSessionId.
	for _, tc := range []struct {
		name   string
		client *testClient
	}{
		{"host", ts.host},
		{"userB", ts.userB},
	} {
		raw, err := tc.client.readUntil("session_redirect")
		if err != nil {
			t.Fatalf("%s readUntil session_redirect: %v", tc.name, err)
		}
		var redirectMessage struct {
			NextSessionID string `json:"nextSessionId"`
		}
		if err := json.Unmarshal(raw, &redirectMessage); err != nil {
			t.Fatalf("%s unmarshal session_redirect: %v", tc.name, err)
		}
		if redirectMessage.NextSessionID != nextID {
			t.Errorf("%s nextSessionId: got %q, want %q", tc.name, redirectMessage.NextSessionID, nextID)
		}
	}
}

// TestRedirectNonexistentSession verifies that redirecting a session that does
// not exist returns 404.
func TestRedirectNonexistentSession(t *testing.T) {
	t.Parallel()

	server, baseURL := startTestServer()
	defer server.Close()

	body, _ := json.Marshal(map[string]any{"nextSessionId": "some-id"})
	response, err := http.Post(baseURL+"/api/sessions/nonexistent/redirect", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST redirect: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", response.StatusCode, http.StatusNotFound)
	}
}

// TestRedirectMissingBody verifies that redirecting without nextSessionId
// returns 400.
func TestRedirectMissingBody(t *testing.T) {
	t.Parallel()

	server, baseURL := startTestServer()
	defer server.Close()

	sessionID, _, err := createSession(baseURL, 10, "host", true)
	if err != nil {
		t.Fatalf("createSession: %v", err)
	}

	body, _ := json.Marshal(map[string]any{})
	response, err := http.Post(baseURL+"/api/sessions/"+sessionID+"/redirect", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST redirect: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", response.StatusCode, http.StatusBadRequest)
	}
}
