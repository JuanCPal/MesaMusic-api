package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"musica-colaborativa-api/internal/models"
	"musica-colaborativa-api/internal/provider"
	"musica-colaborativa-api/internal/queue"
	"musica-colaborativa-api/internal/session"
)

type Handlers struct {
	provider        provider.MusicProvider
	sessions        *session.Manager
	frontendBaseURL string
}

func New(mp provider.MusicProvider, sm *session.Manager, frontendBaseURL string) *Handlers {
	return &Handlers{provider: mp, sessions: sm, frontendBaseURL: frontendBaseURL}
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func trackToSong(track provider.Track) models.Song {
	return models.Song{
		VideoID:         track.ID,
		Title:           track.Title,
		Channel:         track.Channel,
		Thumbnail:       track.Thumbnail,
		DurationSeconds: track.DurationSeconds,
	}
}

func mapProviderError(err error) (status int, message string) {
	switch {
	case errors.Is(err, provider.ErrInvalidQuery):
		return http.StatusBadRequest, "consulta invalida"
	case errors.Is(err, provider.ErrInvalidTrackID):
		return http.StatusBadRequest, "falta videoId"
	case errors.Is(err, provider.ErrTrackNotFound), errors.Is(err, provider.ErrTrackUnavailable):
		return http.StatusBadGateway, "la cancion no esta disponible para reproduccion"
	case errors.Is(err, provider.ErrProviderUnavailable):
		return http.StatusBadGateway, "el proveedor musical no esta disponible, intenta de nuevo"
	default:
		return http.StatusInternalServerError, "error interno del servidor"
	}
}

// GET /api/search?q=texto
func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "falta el parámetro q")
		return
	}

	tracks, err := h.provider.Search(q)
	if err != nil {
		status, message := mapProviderError(err)
		writeError(w, status, message)
		return
	}

	songs := make([]models.Song, 0, len(tracks))
	for _, track := range tracks {
		songs = append(songs, trackToSong(track))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"results": songs})
}

type addToQueueRequest struct {
	VideoID string `json:"videoId"`
}

type createSessionRequest struct {
	Name string `json:"name"`
}

type sessionResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	Status    string    `json:"status"`
	JoinURL   string    `json:"joinUrl"`
}

func (h *Handlers) joinURL(sessionID string) string {
	return fmt.Sprintf("%s/join/%s", h.frontendBaseURL, sessionID)
}

func (h *Handlers) toSessionResponse(s *session.Session) sessionResponse {
	return sessionResponse{
		ID:        s.ID,
		Name:      s.Name,
		CreatedAt: s.CreatedAt,
		Status:    s.Status,
		JoinURL:   h.joinURL(s.ID),
	}
}

// POST /api/sessions { "name": "..." }
func (h *Handlers) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "cuerpo inválido")
		return
	}

	session := h.sessions.Create(req.Name)
	writeJSON(w, http.StatusCreated, h.toSessionResponse(session))
}

// GET /api/sessions/{sessionID}
func (h *Handlers) GetSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	session, ok := h.sessions.Get(sessionID)
	if !ok {
		writeError(w, http.StatusNotFound, "sesion no encontrada")
		return
	}

	writeJSON(w, http.StatusOK, h.toSessionResponse(session))
}

// GET /api/sessions/{sessionID}/qr
func (h *Handlers) GetSessionQR(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	if _, ok := h.sessions.Get(sessionID); !ok {
		writeError(w, http.StatusNotFound, "sesion no encontrada")
		return
	}

	png, err := qrcode.Encode(h.joinURL(sessionID), qrcode.Medium, 256)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo generar el codigo QR")
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

// POST /api/sessions/{sessionID}/queue  { "videoId": "..." }
func (h *Handlers) AddToQueue(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	session, ok := h.sessions.Get(sessionID)
	if !ok {
		writeError(w, http.StatusNotFound, "sesion no encontrada")
		return
	}

	var req addToQueueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "cuerpo inválido")
		return
	}
	if req.VideoID == "" {
		writeError(w, http.StatusBadRequest, "falta videoId")
		return
	}

	// Nunca confiamos en titulo/duracion que mande el cliente: siempre
	// re-consultamos la fuente de verdad del proveedor antes de encolar.
	track, err := h.provider.GetDetails(req.VideoID)
	if err != nil {
		status, message := mapProviderError(err)
		writeError(w, status, message)
		return
	}

	item := session.Queue.Add(trackToSong(*track))
	writeJSON(w, http.StatusCreated, item)
}

// GET /api/sessions/{sessionID}/queue
func (h *Handlers) GetQueue(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	session, ok := h.sessions.Get(sessionID)
	if !ok {
		writeError(w, http.StatusNotFound, "sesion no encontrada")
		return
	}

	writeJSON(w, http.StatusOK, session.Queue.State())
}

// DELETE /api/sessions/{sessionID}/queue/{itemID}
func (h *Handlers) RemoveFromQueue(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	itemID := r.PathValue("itemID")

	session, ok := h.sessions.Get(sessionID)
	if !ok {
		writeError(w, http.StatusNotFound, "sesion no encontrada")
		return
	}

	err := session.Queue.RemoveItem(itemID)
	if err != nil {
		if errors.Is(err, queue.ErrItemNotFound) {
			writeError(w, http.StatusNotFound, "item no encontrado en cola")
			return
		}

		writeError(w, http.StatusInternalServerError, "error interno del servidor")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// POST /api/sessions/{sessionID}/skip
func (h *Handlers) SkipCurrentTrack(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")

	session, ok := h.sessions.Get(sessionID)
	if !ok {
		writeError(w, http.StatusNotFound, "sesion no encontrada")
		return
	}

	session.Queue.SkipCurrent()
	w.WriteHeader(http.StatusNoContent)
}
