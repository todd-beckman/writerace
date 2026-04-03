package handler

import (
	"encoding/json"
	"io/fs"
	"net/http"

	"github.com/todd-beckman/writerace/server/internal/message"
	"github.com/todd-beckman/writerace/server/internal/ratelimit"
	"github.com/todd-beckman/writerace/server/internal/session"
	"github.com/todd-beckman/writerace/server/internal/static"
	"github.com/todd-beckman/writerace/server/internal/ws"
)

// NewMux registers all routes and returns the configured mux.
// If staticFS is non-nil, a catch-all handler serves the embedded frontend
// with SPA fallback. API and WebSocket routes take precedence.
func NewMux(allowedOrigin string, manager *session.Manager, limiter *ratelimit.Limiter, staticFS fs.FS) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/sessions", func(w http.ResponseWriter, r *http.Request) {
		HandleListSessions(manager, w, r)
	})

	mux.Handle("POST /api/sessions", limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		HandleCreateSession(manager, w, r)
	})))

	mux.HandleFunc("POST /api/sessions/{id}/redirect", func(w http.ResponseWriter, r *http.Request) {
		HandleRedirectSession(manager, w, r)
	})

	mux.HandleFunc("GET /ws/{id}", func(w http.ResponseWriter, r *http.Request) {
		ws.HandleWebSocket(allowedOrigin, manager, w, r)
	})

	if staticFS != nil {
		mux.Handle("GET /", static.Handler(staticFS))
	}

	return mux
}

// Cors wraps a handler with CORS headers. Requests from allowedOrigin receive
// the appropriate Access-Control-Allow-* headers; all other origins receive no
// CORS headers, causing browsers to block the request.
func Cors(allowedOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != allowedOrigin {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// SecureHeaders wraps a handler with security-relevant response headers.
func SecureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self' ws: wss:")
		next.ServeHTTP(w, r)
	})
}

func HandleListSessions(manager *session.Manager, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(manager.PublicWaiting())
}

func HandleCreateSession(manager *session.Manager, w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	var req struct {
		Goal     int    `json:"goal"`
		Username string `json:"username"`
		Public   bool   `json:"public"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Goal < 1 {
		http.Error(w, "goal must be a whole number greater than 0", http.StatusBadRequest)
		return
	}
	if msg := session.ValidateUsername(req.Username); msg != "" {
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	newSession, hostID, err := manager.Create(req.Goal, req.Username, req.Public)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id":            newSession.ID,
		"participantId": hostID,
	})
}

func HandleRedirectSession(manager *session.Manager, w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	sessionID := r.PathValue("id")
	foundSession, ok := manager.Get(sessionID)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	var req struct {
		HostParticipantID string `json:"hostParticipantId"`
		NextSessionID     string `json:"nextSessionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.NextSessionID == "" {
		http.Error(w, "nextSessionId is required", http.StatusBadRequest)
		return
	}
	if !foundSession.IsHost(req.HostParticipantID) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	foundSession.Broadcast(message.Build("session_redirect", map[string]any{"nextSessionId": req.NextSessionID}))
	w.WriteHeader(http.StatusNoContent)
}
