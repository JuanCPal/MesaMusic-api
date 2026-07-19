package main

import (
	"log"
	"net/http"

	"musica-colaborativa-api/internal/config"
	"musica-colaborativa-api/internal/handlers"
	"musica-colaborativa-api/internal/models"
	"musica-colaborativa-api/internal/provider"
	"musica-colaborativa-api/internal/session"
	"musica-colaborativa-api/internal/ws"
)

func withCORS(allowedOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func main() {
	cfg := config.Load()

	if cfg.YouTubeAPIKey == "" {
		log.Fatal("YOUTUBE_API_KEY no está configurada. Revisa tu archivo .env")
	}

	musicProvider := provider.NewYouTubeProvider(cfg.YouTubeAPIKey)
	sessionManager := session.NewManager(cfg.BackupPlaylist)
	hub := ws.NewHub(cfg.AllowedOrigins)

	sessionManager.SetOnSessionCreated(func(s *session.Session) {
		s.Queue.SetOnChange(func(state models.QueueState) {
			hub.Broadcast(s.ID, state)
		})
	})

	hub.SetOnEnded(func(sessionID string) {
		if s, ok := sessionManager.Get(sessionID); ok {
			s.Queue.Ended()
		}
	})

	h := handlers.New(musicProvider, sessionManager, cfg.FrontendBaseURL)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/search", h.Search)
	mux.HandleFunc("POST /api/sessions", h.CreateSession)
	mux.HandleFunc("GET /api/sessions/{sessionID}", h.GetSession)
	mux.HandleFunc("GET /api/sessions/{sessionID}/qr", h.GetSessionQR)
	mux.HandleFunc("POST /api/sessions/{sessionID}/queue", h.AddToQueue)
	mux.HandleFunc("GET /api/sessions/{sessionID}/queue", h.GetQueue)
	mux.HandleFunc("GET /api/sessions/{sessionID}/ws", hub.ServeHTTP)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	log.Printf("Servidor escuchando en :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, withCORS(cfg.AllowedOrigins, mux)); err != nil {
		log.Fatal(err)
	}
}
