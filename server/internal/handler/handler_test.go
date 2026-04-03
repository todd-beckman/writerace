package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/todd-beckman/writerace/server/internal/session"
)

func makeSendCh() chan []byte { return make(chan []byte, 32) }

func TestHandleCreateSession(t *testing.T) {
	testCases := []struct {
		name       string
		body       string
		wantStatus int
		wantID     bool // true if response should contain id + participantId
	}{
		{
			name:       "success",
			body:       `{"goal":500,"username":"alice"}`,
			wantStatus: http.StatusOK,
			wantID:     true,
		},
		{
			name:       "invalid JSON body",
			body:       `not-json`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "goal zero",
			body:       `{"goal":0,"username":"alice"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "goal negative",
			body:       `{"goal":-10,"username":"alice"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "username empty",
			body:       `{"goal":500,"username":""}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "username missing",
			body:       `{"goal":500}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager := session.NewManager()
			request := httptest.NewRequest(http.MethodPost, "/api/sessions", strings.NewReader(tc.body))
			recorder := httptest.NewRecorder()

			HandleCreateSession(manager, recorder, request)

			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, tc.wantStatus, recorder.Body.String())
			}

			if tc.wantID {
				var response map[string]string
				if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
					t.Fatalf("response is not valid JSON: %v", err)
				}
				if response["id"] == "" {
					t.Error("response should contain non-empty id")
				}
				if response["participantId"] == "" {
					t.Error("response should contain non-empty participantId")
				}
			}
		})
	}
}

func TestHandleCreateSessionRegistersSession(t *testing.T) {
	manager := session.NewManager()
	request := httptest.NewRequest(http.MethodPost, "/api/sessions", strings.NewReader(`{"goal":100,"username":"alice"}`))
	recorder := httptest.NewRecorder()

	HandleCreateSession(manager, recorder, request)

	var response map[string]string
	json.NewDecoder(recorder.Body).Decode(&response)

	_, found := manager.Get(response["id"])
	if !found {
		t.Error("session should be registered in the manager after creation")
	}
}

func TestHandleListSessions(t *testing.T) {
	t.Run("returns empty array when no sessions exist", func(t *testing.T) {
		manager := session.NewManager()
		request := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
		recorder := httptest.NewRecorder()

		HandleListSessions(manager, recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		var result []session.LobbyEntry
		if err := json.NewDecoder(recorder.Body).Decode(&result); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected 0 sessions, got %d", len(result))
		}
	})

	t.Run("returns only public waiting sessions", func(t *testing.T) {
		manager := session.NewManager()
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

		request := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
		recorder := httptest.NewRecorder()
		HandleListSessions(manager, recorder, request)

		var result []session.LobbyEntry
		json.NewDecoder(recorder.Body).Decode(&result)
		if len(result) != 1 {
			t.Fatalf("expected 1 session, got %d", len(result))
		}
		if result[0].HostUsername != "alice" {
			t.Errorf("HostUsername = %q, want %q", result[0].HostUsername, "alice")
		}
	})

	t.Run("returns correct host username and writer count", func(t *testing.T) {
		manager := session.NewManager()
		newSession, _, _ := manager.Create(1000, "host", true)
		newSession.AddParticipant("writer1", makeSendCh())
		newSession.AddParticipant("writer2", makeSendCh())

		request := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
		recorder := httptest.NewRecorder()
		HandleListSessions(manager, recorder, request)

		var result []session.LobbyEntry
		json.NewDecoder(recorder.Body).Decode(&result)
		if len(result) != 1 {
			t.Fatalf("expected 1 session, got %d", len(result))
		}
		if result[0].HostUsername != "host" {
			t.Errorf("HostUsername = %q, want %q", result[0].HostUsername, "host")
		}
		if result[0].WriterCount != 3 {
			t.Errorf("WriterCount = %d, want 3", result[0].WriterCount)
		}
		if result[0].Goal != 1000 {
			t.Errorf("Goal = %d, want 1000", result[0].Goal)
		}
	})

	t.Run("excludes sessions at MaxParticipants capacity", func(t *testing.T) {
		manager := session.NewManager()
		newSession, _, _ := manager.Create(500, "host", true)
		for i := 1; i < session.MaxParticipants; i++ {
			newSession.AddParticipant(string(rune('a'+i)), makeSendCh())
		}

		request := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
		recorder := httptest.NewRecorder()
		HandleListSessions(manager, recorder, request)

		var result []session.LobbyEntry
		json.NewDecoder(recorder.Body).Decode(&result)
		if len(result) != 0 {
			t.Errorf("expected 0 sessions (at capacity), got %d", len(result))
		}
	})
}

func TestCors(t *testing.T) {
	const allowedOrigin = "http://localhost:5173"

	matchingOriginCases := []struct {
		name         string
		method       string
		wantStatus   int
		wantDelegate bool // true if the inner handler should be called
	}{
		{
			name:         "GET from allowed origin passes through with headers",
			method:       http.MethodGet,
			wantStatus:   http.StatusOK,
			wantDelegate: true,
		},
		{
			name:         "POST from allowed origin passes through with headers",
			method:       http.MethodPost,
			wantStatus:   http.StatusOK,
			wantDelegate: true,
		},
		{
			name:         "OPTIONS preflight from allowed origin returns 204",
			method:       http.MethodOptions,
			wantStatus:   http.StatusNoContent,
			wantDelegate: false,
		},
	}

	for _, tc := range matchingOriginCases {
		t.Run(tc.name, func(t *testing.T) {
			delegateCalled := false
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				delegateCalled = true
				w.WriteHeader(http.StatusOK)
			})

			corsHandler := Cors(allowedOrigin, inner)
			request := httptest.NewRequest(tc.method, "/", nil)
			request.Header.Set("Origin", allowedOrigin)
			recorder := httptest.NewRecorder()

			corsHandler.ServeHTTP(recorder, request)

			if recorder.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", recorder.Code, tc.wantStatus)
			}
			if delegateCalled != tc.wantDelegate {
				t.Errorf("delegateCalled = %v, want %v", delegateCalled, tc.wantDelegate)
			}

			wantHeaders := map[string]string{
				"Access-Control-Allow-Origin":  allowedOrigin,
				"Access-Control-Allow-Methods": "GET, POST, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type",
			}
			for header, want := range wantHeaders {
				if got := recorder.Header().Get(header); got != want {
					t.Errorf("header %q = %q, want %q", header, got, want)
				}
			}
		})
	}

	t.Run("GET from disallowed origin receives no CORS headers", func(t *testing.T) {
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		corsHandler := Cors(allowedOrigin, inner)
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		request.Header.Set("Origin", "http://evil.example.com")
		recorder := httptest.NewRecorder()

		corsHandler.ServeHTTP(recorder, request)

		blockedHeaders := []string{
			"Access-Control-Allow-Origin",
			"Access-Control-Allow-Methods",
			"Access-Control-Allow-Headers",
		}
		for _, header := range blockedHeaders {
			if got := recorder.Header().Get(header); got != "" {
				t.Errorf("header %q = %q, want empty", header, got)
			}
		}
	})
}
