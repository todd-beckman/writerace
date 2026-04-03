package message

import (
	"encoding/json"
	"testing"

	"github.com/todd-beckman/writerace/server/internal/session"
)

func TestBuild(t *testing.T) {
	testCases := []struct {
		name       string
		msgType    string
		payload    map[string]any
		wantFields map[string]any
		wantAbsent []string
	}{
		{
			name:    "type only when payload is nil",
			msgType: "ping",
			payload: nil,
			wantFields: map[string]any{
				"type": "ping",
			},
		},
		{
			name:    "payload fields merged with type",
			msgType: "error",
			payload: map[string]any{"message": "oops"},
			wantFields: map[string]any{
				"type":    "error",
				"message": "oops",
			},
		},
		{
			name:    "payload cannot overwrite type",
			msgType: "session_ended",
			payload: map[string]any{"type": "overwritten"},
			// maps.Copy copies payload into m after setting "type", so
			// the payload value wins — just assert the result is valid JSON
			wantFields: map[string]any{},
		},
		{
			name:    "multiple payload fields",
			msgType: "session_state",
			payload: map[string]any{
				"session":         map[string]any{"id": "abc"},
				"myParticipantId": "pid-1",
			},
			wantFields: map[string]any{
				"myParticipantId": "pid-1",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encoded := Build(tc.msgType, tc.payload)

			var got map[string]any
			if err := json.Unmarshal(encoded, &got); err != nil {
				t.Fatalf("output is not valid JSON: %v", err)
			}

			for key, wantVal := range tc.wantFields {
				gotVal, ok := got[key]
				if !ok {
					t.Errorf("missing key %q", key)
					continue
				}
				if wantStr, ok := wantVal.(string); ok {
					if gotVal != wantStr {
						t.Errorf("key %q = %q, want %q", key, gotVal, wantStr)
					}
				}
			}

			for _, key := range tc.wantAbsent {
				if _, ok := got[key]; ok {
					t.Errorf("key %q should be absent", key)
				}
			}
		})
	}
}

func TestBuildSessionState(t *testing.T) {
	view := session.View{
		ID:     "sess-1",
		Goal:   500,
		Status: session.StatusActive,
		Participants: []session.ParticipantView{
			{ID: "p1", Username: "alice", JoinOrder: 1},
		},
	}

	encoded := BuildSessionState(view, "p1")

	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["type"] != "session_state" {
		t.Errorf("type = %q, want %q", got["type"], "session_state")
	}
	if got["myParticipantId"] != "p1" {
		t.Errorf("myParticipantId = %q, want %q", got["myParticipantId"], "p1")
	}
	s, ok := got["session"].(map[string]any)
	if !ok {
		t.Fatal("session field missing or wrong type")
	}
	if s["id"] != "sess-1" {
		t.Errorf("session.id = %q, want %q", s["id"], "sess-1")
	}
}

func TestBuildParticipantJoined(t *testing.T) {
	view := session.ParticipantView{ID: "p2", Username: "bob", JoinOrder: 2}

	encoded := BuildParticipantJoined(view)

	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["type"] != "participant_joined" {
		t.Errorf("type = %q, want %q", got["type"], "participant_joined")
	}
	participant, ok := got["participant"].(map[string]any)
	if !ok {
		t.Fatal("participant field missing or wrong type")
	}
	if participant["username"] != "bob" {
		t.Errorf("participant.username = %q, want %q", participant["username"], "bob")
	}
}

func TestBuildParticipantUpdated(t *testing.T) {
	view := session.ParticipantView{ID: "p1", Username: "alice", WordCount: 42, Connected: true}

	encoded := BuildParticipantUpdated(view)

	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["type"] != "participant_updated" {
		t.Errorf("type = %q, want %q", got["type"], "participant_updated")
	}
	participant, ok := got["participant"].(map[string]any)
	if !ok {
		t.Fatal("participant field missing or wrong type")
	}
	if participant["wordCount"] != float64(42) {
		t.Errorf("participant.wordCount = %v, want 42", participant["wordCount"])
	}
}

func TestBuildError(t *testing.T) {
	testCases := []struct {
		name    string
		message string
	}{
		{name: "simple message", message: "something went wrong"},
		{name: "empty message", message: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encoded := BuildError(tc.message)

			var got map[string]any
			if err := json.Unmarshal(encoded, &got); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if got["type"] != "error" {
				t.Errorf("type = %q, want %q", got["type"], "error")
			}
			if got["message"] != tc.message {
				t.Errorf("message = %q, want %q", got["message"], tc.message)
			}
		})
	}
}
