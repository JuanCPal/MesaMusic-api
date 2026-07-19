package provider

import "errors"

// Track representa una canción normalizada en el dominio de MesaMusic,
// independiente del formato crudo que devuelva cada proveedor.
type Track struct {
	ID              string
	Title           string
	Channel         string
	Thumbnail       string
	DurationSeconds int
}

// MusicProvider define el contrato mínimo que necesita la aplicación
// para buscar canciones y resolver detalles confiables al encolar.
type MusicProvider interface {
	Search(query string) ([]Track, error)
	GetDetails(trackID string) (*Track, error)
}

var (
	ErrInvalidQuery       = errors.New("invalid query")
	ErrInvalidTrackID     = errors.New("invalid track id")
	ErrProviderUnavailable = errors.New("provider unavailable")
	ErrTrackNotFound      = errors.New("track not found")
	ErrTrackUnavailable   = errors.New("track unavailable")
)
