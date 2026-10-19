package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"musica-colaborativa-api/internal/models"
	"musica-colaborativa-api/internal/provider"
	"musica-colaborativa-api/internal/session"
)

type noopProvider struct{}

func (noopProvider) Search(string) ([]provider.Track, error) {
	return nil, nil
}

func (noopProvider) GetDetails(string) (*provider.Track, error) {
	return nil, nil
}

func newTestMuxAndSession() (*Handlers, *session.Session, *http.ServeMux) {
	sm := session.NewManager([]string{"backup-1"})
	h := New(noopProvider{}, sm, "http://localhost:3000")
	s := sm.Create("Sesion Test", "")

	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/sessions/{sessionID}/queue/{itemID}", h.RemoveFromQueue)
	mux.HandleFunc("POST /api/sessions/{sessionID}/skip", h.SkipCurrentTrack)
	mux.HandleFunc("GET /api/sessions/{sessionID}/queue", h.GetQueue)

	return h, s, mux
}

func queueStateFromGet(t *testing.T, mux *http.ServeMux, sessionID string) models.QueueState {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/"+sessionID+"/queue", nil)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected GET queue status %d, got %d", http.StatusOK, res.Code)
	}

	var state models.QueueState
	if err := json.NewDecoder(res.Body).Decode(&state); err != nil {
		t.Fatalf("decoding queue state: %v", err)
	}

	return state
}

func TestRemoveFromQueueReturns204AndRemovesItem(t *testing.T) {
	_, s, mux := newTestMuxAndSession()

	_ = s.Queue.Add(models.Song{VideoID: "np-1", Title: "Now", DurationSeconds: 180}, "Ana")
	toRemove := s.Queue.Add(models.Song{VideoID: "q-1", Title: "Q1", DurationSeconds: 120}, "Ana")
	remaining := s.Queue.Add(models.Song{VideoID: "q-2", Title: "Q2", DurationSeconds: 150}, "Ana")

	req := httptest.NewRequest(http.MethodDelete, "/api/sessions/"+s.ID+"/queue/"+toRemove.ID, nil)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, res.Code)
	}

	state := queueStateFromGet(t, mux, s.ID)
	if len(state.Queue) != 1 {
		t.Fatalf("expected 1 pending item, got %d", len(state.Queue))
	}
	if state.Queue[0].ID != remaining.ID {
		t.Fatalf("expected remaining item %q, got %q", remaining.ID, state.Queue[0].ID)
	}
}

func TestRemoveFromQueueReturns404ForMissingItem(t *testing.T) {
	_, s, mux := newTestMuxAndSession()

	_ = s.Queue.Add(models.Song{VideoID: "np-1", Title: "Now", DurationSeconds: 180}, "Ana")
	pending := s.Queue.Add(models.Song{VideoID: "q-1", Title: "Q1", DurationSeconds: 120}, "Ana")

	req := httptest.NewRequest(http.MethodDelete, "/api/sessions/"+s.ID+"/queue/missing-item", nil)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, res.Code)
	}

	var payload map[string]string
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decoding error payload: %v", err)
	}

	if payload["error"] == "" {
		t.Fatal("expected non-empty error message")
	}

	state := queueStateFromGet(t, mux, s.ID)
	if len(state.Queue) != 1 {
		t.Fatalf("expected queue unchanged with 1 pending item, got %d", len(state.Queue))
	}
	if state.Queue[0].ID != pending.ID {
		t.Fatalf("expected pending item %q, got %q", pending.ID, state.Queue[0].ID)
	}
}

func TestRemoveFromQueueReturns404ForMissingSession(t *testing.T) {
	_, _, mux := newTestMuxAndSession()

	req := httptest.NewRequest(http.MethodDelete, "/api/sessions/missing-session/queue/item-1", nil)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, res.Code)
	}
}

func TestSkipCurrentTrackReturns204AndAdvancesNowPlaying(t *testing.T) {
	_, s, mux := newTestMuxAndSession()

	_ = s.Queue.Add(models.Song{VideoID: "np-1", Title: "Now", DurationSeconds: 180}, "Ana")
	next := s.Queue.Add(models.Song{VideoID: "q-1", Title: "Q1", DurationSeconds: 120}, "Ana")

	req := httptest.NewRequest(http.MethodPost, "/api/sessions/"+s.ID+"/skip", nil)
	res := httptest.NewRecorder()
	mux.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, res.Code)
	}

	state := queueStateFromGet(t, mux, s.ID)
	if state.NowPlaying == nil {
		t.Fatal("expected nowPlaying after skip")
	}
	if state.NowPlaying.ID != next.ID {
		t.Fatalf("expected nowPlaying item %q, got %q", next.ID, state.NowPlaying.ID)
	}
}

type fakeProvider struct{ noopProvider }

func (fakeProvider) GetDetails(id string) (*provider.Track, error) {
return &provider.Track{ID: id, Title: "T", DurationSeconds: 100}, nil
}

func newAPIMux() (*http.ServeMux, *session.Manager) {
sm := session.NewManager(nil)
h := New(fakeProvider{}, sm, "http://localhost:3000")
mux := http.NewServeMux()
mux.HandleFunc("POST /api/sessions", h.CreateSession)
mux.HandleFunc("GET /api/sessions/{sessionID}", h.GetSession)
mux.HandleFunc("POST /api/sessions/{sessionID}/queue", h.AddToQueue)
return mux, sm
}

func doJSON(mux *http.ServeMux, method, path, body string) *httptest.ResponseRecorder {
req := httptest.NewRequest(method, path, strings.NewReader(body))
res := httptest.NewRecorder()
mux.ServeHTTP(res, req)
return res
}

func TestCreateSessionType(t *testing.T) {
cases := []struct {
name     string
body     string
status   int
wantType string
}{
{"default", `{"name":"x"}`, http.StatusCreated, "meeting"},
{"business", `{"name":"x","type":"business"}`, http.StatusCreated, "business"},
{"invalid", `{"name":"x","type":"party"}`, http.StatusBadRequest, ""},
}

for _, tc := range cases {
t.Run(tc.name, func(t *testing.T) {
mux, _ := newAPIMux()
res := doJSON(mux, http.MethodPost, "/api/sessions", tc.body)
if res.Code != tc.status {
t.Fatalf("expected status %d, got %d", tc.status, res.Code)
}
if tc.status != http.StatusCreated {
return
}

var created sessionResponse
if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
t.Fatal(err)
}
if created.Type != tc.wantType {
t.Fatalf("expected type %q, got %q", tc.wantType, created.Type)
}

got := doJSON(mux, http.MethodGet, "/api/sessions/"+created.ID, "")
var fetched sessionResponse
if err := json.NewDecoder(got.Body).Decode(&fetched); err != nil {
t.Fatal(err)
}
if fetched.Type != tc.wantType {
t.Fatalf("GET: expected type %q, got %q", tc.wantType, fetched.Type)
}
})
}
}

func TestAddToQueueRequestedBy(t *testing.T) {
cases := []struct {
name string
body string
want string
}{
{"given", `{"videoId":"v","requestedBy":"Ana"}`, "Ana"},
{"trimmed", `{"videoId":"v","requestedBy":"  Ana  "}`, "Ana"},
{"missing", `{"videoId":"v"}`, "Alguien"},
{"blank", `{"videoId":"v","requestedBy":"   "}`, "Alguien"},
{"truncated", `{"videoId":"v","requestedBy":"` + strings.Repeat("a", 50) + `"}`, strings.Repeat("a", 30)},
}

for _, tc := range cases {
t.Run(tc.name, func(t *testing.T) {
mux, sm := newAPIMux()
s := sm.Create("x", "")

res := doJSON(mux, http.MethodPost, "/api/sessions/"+s.ID+"/queue", tc.body)
if res.Code != http.StatusCreated {
t.Fatalf("expected status %d, got %d", http.StatusCreated, res.Code)
}

var item models.QueueItem
if err := json.NewDecoder(res.Body).Decode(&item); err != nil {
t.Fatal(err)
}
if item.RequestedBy != tc.want {
t.Fatalf("expected RequestedBy %q, got %q", tc.want, item.RequestedBy)
}
})
}
}
