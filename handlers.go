package handlers

import (
	"encoding/json"
	"net/http"

	"musica-colaborativa-api/internal/queue"
	"musica-colaborativa-api/internal/youtube"
)

type Handlers struct {
	yt    *youtube.Client
	queue *queue.Manager
}

func New(yt *youtube.Client, q *queue.Manager) *Handlers {
	return &Handlers{yt: yt, queue: q}
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// GET /api/search?q=texto
func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "falta el parámetro q")
		return
	}

	songs, err := h.yt.Search(q)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
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

	// Nunca confiamos en título/duración que mande el cliente: siempre
	// re-consultamos la fuente de verdad (YouTube) antes de encolar.
	song, err := h.yt.GetVideoDetails(req.VideoID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	item := h.queue.Add(*song, req.Mesa)
	writeJSON(w, http.StatusCreated, item)
}

// GET /api/queue
func (h *Handlers) GetQueue(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.queue.State())
}
