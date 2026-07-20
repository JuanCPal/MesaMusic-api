package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"musica-colaborativa-api/internal/models"
)

func TestServeHTTPRejectsMissingSessionID(t *testing.T) {
	hub := NewHub("http://localhost:3000")
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	res := httptest.NewRecorder()

	hub.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
}

func TestBroadcastOnlyReachesClientsInTargetSession(t *testing.T) {
	hub := NewHub("http://localhost:3000")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ws/{sessionID}", hub.ServeHTTP)
	server := httptest.NewServer(mux)
	defer server.Close()

	baseURL := "ws" + strings.TrimPrefix(server.URL, "http")
	sessionAConn := mustDialWebSocket(t, baseURL+"/ws/session-a")
	defer sessionAConn.Close()
	sessionBConn := mustDialWebSocket(t, baseURL+"/ws/session-b")
	defer sessionBConn.Close()

	state := models.QueueState{
		Queue: []models.QueueItem{{
			ID: "item-1",
			Song: models.Song{
				VideoID: "video-1",
				Title:   "Tema 1",
			},
		}},
	}

	hub.Broadcast("session-a", state)

	var got map[string]json.RawMessage
	if err := sessionAConn.ReadJSON(&got); err != nil {
		t.Fatalf("expected broadcast for session-a client: %v", err)
	}

	if gotType := string(got["type"]); gotType != "\"queueState\"" {
		t.Fatalf("expected queueState payload type, got %s", gotType)
	}

	if err := sessionBConn.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("setting read deadline: %v", err)
	}

	var unexpected map[string]json.RawMessage
	err := sessionBConn.ReadJSON(&unexpected)
	if err == nil {
		t.Fatal("expected no broadcast for session-b client")
	}

	if !websocket.IsUnexpectedCloseError(err) {
		if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
			return
		}
	}

	t.Fatalf("expected timeout while waiting for unrelated session broadcast, got %v", err)
}

func mustDialWebSocket(t *testing.T, url string) *websocket.Conn {
	t.Helper()

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dialing websocket %s: %v", url, err)
	}

	return conn
}
