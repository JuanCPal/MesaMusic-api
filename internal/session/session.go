package session

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"musica-colaborativa-api/internal/queue"
)

const (
	StatusActive = "active"
	StatusEnded  = "ended"
)

type Session struct {
	ID        string
	Name      string
	CreatedAt time.Time
	Status    string
	Queue     *queue.Manager `json:"-"`
}

// genID genera un identificador corto y suficientemente unico para una
// sesion, sin depender de ninguna libreria externa de UUID.
func genID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
