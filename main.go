package main

import (
	"log"
	"net/http"

	"musica-colaborativa-api/internal/config"
	"musica-colaborativa-api/internal/handlers"
	"musica-colaborativa-api/internal/queue"
	"musica-colaborativa-api/internal/ws"
	"musica-colaborativa-api/internal/youtube"
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

	ytClient := youtube.NewClient(cfg.YouTubeAPIKey)
	hub := ws.NewHub(cfg.AllowedOrigins)
	queueManager := queue.NewManager(cfg.BackupPlaylist)

	// Conectamos las piezas: cuando la cola cambia, el hub hace broadcast;
	// cuando el panel avisa que una canción terminó, la cola avanza.
	queueManager.SetOnChange(hub.Broadcast)
	hub.SetOnEnded(queueManager.Ended)

	h := handlers.New(ytClient, queueManager)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/search", h.Search)
	mux.HandleFunc("POST /api/queue", h.AddToQueue)
	mux.HandleFunc("GET /api/queue", h.GetQueue)
	mux.HandleFunc("GET /ws", hub.ServeHTTP)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	log.Printf("Servidor escuchando en :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, withCORS(cfg.AllowedOrigins, mux)); err != nil {
		log.Fatal(err)
	}
}
