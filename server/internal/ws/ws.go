package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/todd-beckman/writerace/server/internal/message"
	"github.com/todd-beckman/writerace/server/internal/session"
)

const (
	sendBufSize    = 32
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

// HandleWebSocket upgrades the HTTP connection and manages the participant lifecycle.
// Only connections whose Origin header matches allowedOrigin are accepted.
func HandleWebSocket(allowedOrigin string, manager *session.Manager, w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	currentSession, ok := manager.Get(sessionID)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return r.Header.Get("Origin") == allowedOrigin
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}

	conn.SetReadLimit(maxMessageSize)

	// The first message must be join or rejoin.
	var first message.ClientMsg
	conn.SetReadDeadline(time.Now().Add(pongWait))
	if err := conn.ReadJSON(&first); err != nil {
		conn.Close()
		return
	}
	conn.SetReadDeadline(time.Time{}) // clear deadline; ping/pong keeps the connection alive

	sendCh := make(chan []byte, sendBufSize)

	var participant *session.Participant

	switch first.Type {
	case "join":
		if msg := session.ValidateUsername(first.Username); msg != "" {
			conn.WriteMessage(websocket.TextMessage, message.BuildError(msg))
			conn.Close()
			return
		}
		joinedParticipant, err := currentSession.AddParticipant(first.Username, sendCh)
		if err != nil {
			conn.WriteMessage(websocket.TextMessage, message.BuildError(err.Error()))
			conn.Close()
			return
		}
		participant = joinedParticipant

		conn.WriteMessage(websocket.TextMessage, message.BuildSessionState(currentSession.View(), participant.ID))

		currentSession.BroadcastExcept(message.BuildParticipantJoined(session.ParticipantView{
			ID:          participant.ID,
			Username:    participant.Username,
			WordCount:   participant.WordCount,
			Connected:   participant.Connected,
			Completed:   participant.Completed,
			JoinOrder:   participant.JoinOrder,
			FinishOrder: participant.FinishOrder,
		}), participant.ID)

	case "rejoin":
		if first.ParticipantID == "" {
			conn.WriteMessage(websocket.TextMessage, message.BuildError("participantId is required"))
			conn.Close()
			return
		}
		rejoinedParticipant, err := currentSession.Reconnect(first.ParticipantID, sendCh)
		if err != nil {
			conn.WriteMessage(websocket.TextMessage, message.BuildError(err.Error()))
			conn.Close()
			return
		}
		participant = rejoinedParticipant

		conn.WriteMessage(websocket.TextMessage, message.BuildSessionState(currentSession.View(), participant.ID))

		currentSession.BroadcastExcept(message.BuildParticipantUpdated(session.ParticipantView{
			ID:          participant.ID,
			Username:    participant.Username,
			WordCount:   participant.WordCount,
			Connected:   true,
			Completed:   participant.Completed,
			JoinOrder:   participant.JoinOrder,
			FinishOrder: participant.FinishOrder,
		}), participant.ID)

	default:
		conn.WriteMessage(websocket.TextMessage, message.BuildError("first message must be join or rejoin"))
		conn.Close()
		return
	}

	// Start the write pump in the background; read pump runs on this goroutine.
	go writePump(conn, sendCh)
	readPump(conn, sendCh, currentSession, participant)
}

// readPump drives the participant's connection until it closes.
func readPump(conn *websocket.Conn, sendCh chan []byte, currentSession *session.Session, participant *session.Participant) {
	defer func() {
		// On exit: mark disconnected (which also closes sendCh, signalling the
		// writePump to drain and exit), then broadcast the departure.
		// conn.Close() is intentionally omitted — the writePump closes the
		// connection after draining sendCh.
		currentSession.Disconnect(participant.ID)
		currentSession.EndIfHostLeft(participant)
		currentSession.BroadcastExcept(message.BuildParticipantUpdated(session.ParticipantView{
			ID:          participant.ID,
			Username:    participant.Username,
			WordCount:   participant.WordCount,
			Connected:   false,
			Completed:   participant.Completed,
			JoinOrder:   participant.JoinOrder,
			FinishOrder: participant.FinishOrder,
		}), participant.ID)
	}()

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var clientMessage message.ClientMsg
		if err := json.Unmarshal(raw, &clientMessage); err != nil {
			continue
		}

		switch clientMessage.Type {
		case "start":
			if participant.JoinOrder != 1 {
				currentSession.SendTo(participant.ID, message.BuildError("only the host can start the session"))
				continue
			}
			if !currentSession.Start() {
				continue
			}
			currentSession.Broadcast(message.Build("session_started", nil))

		case "update":
			if clientMessage.WordCount < 0 {
				continue
			}
			updatedParticipant, sessionEnded := currentSession.UpdateWordCount(participant.ID, clientMessage.WordCount)
			currentSession.Broadcast(message.BuildParticipantUpdated(updatedParticipant))
			if sessionEnded {
				currentSession.Broadcast(message.Build("session_ended", nil))
				return
			}

		case "end":
			currentSession.End()
			currentSession.Broadcast(message.Build("session_ended", nil))
			return

		case "ping":
			currentSession.SendTo(participant.ID, message.Build("pong", nil))
		}
	}
}

// writePump drains the send channel and writes to the WebSocket.
func writePump(conn *websocket.Conn, sendCh <-chan []byte) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()

	for {
		select {
		case outgoingMessage, ok := <-sendCh:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, outgoingMessage); err != nil {
				return
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
