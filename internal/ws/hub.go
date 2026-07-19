package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"

	"musica-colaborativa-api/internal/models"
)

// EndedCallback se invoca cuando el panel de reproducción avisa que la
// canción actual terminó, para que el manager de cola avance.
type EndedCallback func(sessionID string)

type Hub struct {
	mu      sync.Mutex
	clients map[string]map[*websocket.Conn]bool

	upgrader websocket.Upgrader
	onEnded  EndedCallback
}

func NewHub(allowedOrigin string) *Hub {
	return &Hub{
		clients: make(map[string]map[*websocket.Conn]bool),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// En desarrollo aceptamos cualquier origen; ajusta esto si
				// despliegas a producción con un dominio fijo.
				return true
			},
		},
	}
}

// SetOnEnded conecta el hub con el manager de cola.
func (h *Hub) SetOnEnded(cb EndedCallback) {
	h.onEnded = cb
}

// ServeHTTP maneja el upgrade de la conexión y el loop de lectura de cada cliente.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("sessionID")
	if sessionID == "" {
		http.Error(w, "missing sessionID", http.StatusBadRequest)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("error en upgrade de websocket:", err)
		return
	}

	h.mu.Lock()
	if h.clients[sessionID] == nil {
		h.clients[sessionID] = make(map[*websocket.Conn]bool)
	}
	h.clients[sessionID][conn] = true
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.clients[sessionID], conn)
		if len(h.clients[sessionID]) == 0 {
			delete(h.clients, sessionID)
		}
		h.mu.Unlock()
		conn.Close()
	}()

	for {
		var msg struct {
			Type string `json:"type"`
		}
		if err := conn.ReadJSON(&msg); err != nil {
			break // el cliente cerró la conexión o hubo error de red
		}

		if msg.Type == "ended" && h.onEnded != nil {
			h.onEnded(sessionID)
		}
	}
}

// Broadcast envía el estado actual de la cola a todos los clientes conectados.
// Se usa como el OnChange callback del queue.Manager.
func (h *Hub) Broadcast(sessionID string, state models.QueueState) {
	payload, err := json.Marshal(map[string]interface{}{
		"type":  "queueState",
		"state": state,
	})
	if err != nil {
		log.Println("error serializando estado de cola:", err)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	for conn := range h.clients[sessionID] {
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			conn.Close()
			delete(h.clients[sessionID], conn)
			if len(h.clients[sessionID]) == 0 {
				delete(h.clients, sessionID)
			}
		}
	}
}
