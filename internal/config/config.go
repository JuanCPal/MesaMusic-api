package config

import (
	"bufio"
	"os"
	"strings"
)

// Config contiene toda la configuración de la aplicación cargada desde
// variables de entorno (o un archivo .env en desarrollo).
type Config struct {
	Port            string
	YouTubeAPIKey   string
	BackupPlaylist  []string // IDs de video de YouTube para reproducir cuando la cola esté vacía
	AllowedOrigins  string   // origen permitido para CORS, "*" en desarrollo
	FrontendBaseURL string   // URL base del frontend para construir links de join y QR
}

// Load lee un archivo .env (si existe) y luego las variables de entorno,
// dando prioridad a las variables de entorno reales sobre el .env.
func Load() *Config {
	loadDotEnv(".env")

	cfg := &Config{
		Port:            getEnv("PORT", "8080"),
		YouTubeAPIKey:   getEnv("YOUTUBE_API_KEY", ""),
		AllowedOrigins:  getEnv("ALLOWED_ORIGINS", "*"),
		FrontendBaseURL: getEnv("FRONTEND_BASE_URL", "http://localhost:3000"),
	}

	backup := getEnv("BACKUP_PLAYLIST", "")
	if backup != "" {
		for _, id := range strings.Split(backup, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				cfg.BackupPlaylist = append(cfg.BackupPlaylist, id)
			}
		}
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// loadDotEnv es un parser minimalista de archivos .env (KEY=VALUE por línea).
// No es necesaria ninguna dependencia externa para esto.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // no pasa nada si no existe, se usarán variables de entorno reales
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		// Solo seteamos si no existe ya como variable de entorno real
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
}
