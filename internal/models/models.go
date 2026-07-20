package models

import "time"

// Song representa una canción tal como viene de YouTube, ya normalizada.
type Song struct {
	VideoID         string `json:"videoId"`
	Title           string `json:"title"`
	Channel         string `json:"channel"`
	Thumbnail       string `json:"thumbnail"`
	DurationSeconds int    `json:"durationSeconds"`
}

// QueueItem es una canción ya agregada a la cola compartida, con metadata
// adicional propia del sistema de colas.
type QueueItem struct {
	ID       string    `json:"id"` // identificador único de esta entrada en la cola
	Song     Song      `json:"song"`
	AddedAt  time.Time `json:"addedAt"`
	IsBackup bool      `json:"isBackup"` // true si viene de la playlist de respaldo, no de un pedido real
}

// QueueState es el snapshot completo que se envía a los clientes (REST y WS).
type QueueState struct {
	NowPlaying        *QueueItem  `json:"nowPlaying"`
	Queue             []QueueItem `json:"queue"`
	EstimatedWaitSecs []int       `json:"estimatedWaitSecs"` // tiempo estimado de espera por cada item en Queue, mismo índice
}
