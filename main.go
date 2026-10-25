package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"musica-colaborativa-api/internal/config"
	"musica-colaborativa-api/internal/db"
	"musica-colaborativa-api/internal/handlers"
	"musica-colaborativa-api/internal/models"
	"musica-colaborativa-api/internal/provider"
	"musica-colaborativa-api/internal/session"
	"musica-colaborativa-api/internal/ws"
)

func withCORS(allowedOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
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

	musicProvider := provider.NewCachedProvider(provider.NewYouTubeProvider(cfg.YouTubeAPIKey))

	var database *db.DB
	if cfg.DatabaseURL == "" {
		log.Println("DATABASE_URL vacía: arrancando en modo memoria (sin persistencia)")
	} else {
		if cfg.DBAutoMigrate {
			migURL := cfg.DatabaseMigrateURL
			if migURL == "" {
				migURL = cfg.DatabaseURL
			}
			log.Println("Aplicando migraciones...")
			if err := db.Migrate(migURL); err != nil {
				log.Fatalf("migraciones fallaron: %v", err)
			}
			log.Println("Migraciones al día")
		}
		var err error
		database, err = db.Connect(context.Background(), cfg.DatabaseURL, db.Options{MaxConns: cfg.DBMaxConns, MinConns: 1})
		if err != nil {
			log.Fatalf("no se pudo conectar a la base de datos: %v", err)
		}
		log.Println("Conectado a PostgreSQL")
	}
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
	mux.HandleFunc("DELETE /api/sessions/{sessionID}/queue/{itemID}", h.RemoveFromQueue)
	mux.HandleFunc("POST /api/sessions/{sessionID}/skip", h.SkipCurrentTrack)
	mux.HandleFunc("GET /api/sessions/{sessionID}/ws", hub.ServeHTTP)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		if database == nil {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			return
		}
		// Siempre 200: una BD caída no debe hacer que Render reinicie el servicio,
		// el estado se reporta en el cuerpo.
		status, dbStatus := "ok", "up"
		if err := database.Ping(r.Context(), 2*time.Second); err != nil {
			log.Printf("health: ping a la BD falló: %v", err)
			status, dbStatus = "degraded", "down"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": status, "db": dbStatus})
	})

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: withCORS(cfg.AllowedOrigins, mux)}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("Servidor escuchando en :%s", cfg.Port)
		errCh <- srv.ListenAndServe()
	}()

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		if database != nil {
			database.Close()
		}
		log.Fatal(err)
	case <-sigCtx.Done():
		log.Println("Apagando servidor...")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("error en shutdown: %v", err)
	}
	if database != nil {
		database.Close()
	}
}
