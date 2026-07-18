package queue

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"musica-colaborativa-api/internal/models"
)

// genID genera un identificador corto y suficientemente único para una
// entrada de cola, sin depender de ninguna librería externa de UUID.
func genID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// OnChange se llama cada vez que el estado de la cola cambia, para que quien
// esté escuchando (el hub de WebSockets) pueda hacer broadcast.
type OnChange func(state models.QueueState)

// Manager mantiene el estado de la cola en memoria, protegido por mutex.
type Manager struct {
	mu         sync.Mutex
	nowPlaying *models.QueueItem
	pending    []models.QueueItem

	backupPlaylist []string
	backupIndex    int

	onChange OnChange
}

func NewManager(backupPlaylist []string) *Manager {
	return &Manager{
		pending:        make([]models.QueueItem, 0),
		backupPlaylist: backupPlaylist,
	}
}

// SetOnChange registra el callback de notificación (lo llama main.go una vez,
// conectándolo con el hub de WebSockets).
func (m *Manager) SetOnChange(cb OnChange) {
	m.onChange = cb
}

// Add agrega una canción pedida por un cliente al final de la cola.
func (m *Manager) Add(song models.Song, mesa string) models.QueueItem {
	m.mu.Lock()
	item := models.QueueItem{
		ID:      genID(),
		Song:    song,
		Mesa:    mesa,
		AddedAt: time.Now(),
	}
	m.pending = append(m.pending, item)

	// Si no hay nada sonando (ej. estaba reproduciendo respaldo o estaba vacío
	// del todo), arrancamos esta directamente.
	shouldAdvance := m.nowPlaying == nil
	m.mu.Unlock()

	if shouldAdvance {
		m.advance()
	} else {
		m.notify()
	}

	return item
}

// Ended se llama cuando el panel avisa que la canción actual terminó.
// Avanza la cola al siguiente elemento (o a la playlist de respaldo si está vacía).
func (m *Manager) Ended() {
	m.advance()
}

// advance mueve pending[0] a nowPlaying, o si no hay pendientes, toma el
// siguiente video de la playlist de respaldo.
func (m *Manager) advance() {
	m.mu.Lock()

	if len(m.pending) > 0 {
		next := m.pending[0]
		m.pending = m.pending[1:]
		m.nowPlaying = &next
	} else if len(m.backupPlaylist) > 0 {
		videoID := m.backupPlaylist[m.backupIndex%len(m.backupPlaylist)]
		m.backupIndex++
		m.nowPlaying = &models.QueueItem{
			ID:       genID(),
			Song:     models.Song{VideoID: videoID, Title: "Reproduciendo playlist de respaldo"},
			AddedAt:  time.Now(),
			IsBackup: true,
		}
	} else {
		m.nowPlaying = nil
	}

	m.mu.Unlock()
	m.notify()
}

// State devuelve una copia del estado actual, con tiempo estimado de espera
// calculado para cada canción en cola.
func (m *Manager) State() models.QueueState {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := models.QueueState{
		NowPlaying:        m.nowPlaying,
		Queue:             append([]models.QueueItem{}, m.pending...),
		EstimatedWaitSecs: make([]int, len(m.pending)),
	}

	cumulative := 0
	if m.nowPlaying != nil {
		cumulative = m.nowPlaying.Song.DurationSeconds
	}
	for i, item := range m.pending {
		state.EstimatedWaitSecs[i] = cumulative
		cumulative += item.Song.DurationSeconds
	}

	return state
}

func (m *Manager) notify() {
	if m.onChange != nil {
		m.onChange(m.State())
	}
}
