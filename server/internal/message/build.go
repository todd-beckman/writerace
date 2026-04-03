package message

import (
	"encoding/json"
	"maps"

	"github.com/todd-beckman/writerace/server/internal/session"
)

// ClientMsg is the structure of messages sent from client to server.
type ClientMsg struct {
	Type          string `json:"type"`
	Username      string `json:"username,omitempty"`
	ParticipantID string `json:"participantId,omitempty"`
	WordCount     int    `json:"wordCount,omitempty"`
}

func Build(msgType string, payload map[string]any) []byte {
	result := map[string]any{"type": msgType}
	maps.Copy(result, payload)
	encoded, _ := json.Marshal(result)
	return encoded
}

func BuildSessionState(view session.View, myParticipantID string) []byte {
	return Build("session_state", map[string]any{
		"session":         view,
		"myParticipantId": myParticipantID,
	})
}

func BuildParticipantJoined(view session.ParticipantView) []byte {
	return Build("participant_joined", map[string]any{"participant": view})
}

func BuildParticipantUpdated(view session.ParticipantView) []byte {
	return Build("participant_updated", map[string]any{"participant": view})
}

func BuildError(message string) []byte {
	return Build("error", map[string]any{"message": message})
}
