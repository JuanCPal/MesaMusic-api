package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"musica-colaborativa-api/internal/models"
	"musica-colaborativa-api/internal/provider"
	"musica-colaborativa-api/internal/queue"
)

type Handlers struct {
	provider provider.MusicProvider
	queue *queue.Manager
}

func New(mp provider.MusicProvider, q *queue.Manager) *Handlers {
	return &Handlers{provider: mp, queue: q}
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
	Mesa    string `json:"mesa"`
}

// POST /api/queue  { "videoId": "...", "mesa": "opcional" }
func (h *Handlers) AddToQueue(w http.ResponseWriter, r *http.Request) {
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

	item := h.queue.Add(trackToSong(*track), req.Mesa)
	writeJSON(w, http.StatusCreated, item)
}

// GET /api/queue
func (h *Handlers) GetQueue(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.queue.State())
}
