# Música Colaborativa API

Backend en Go para el sistema de cola de música compartida vía QR. Sin base
de datos por ahora: todo el estado vive en memoria mientras el proceso corre
(pensado para una reunión/evento puntual, no para persistencia a largo plazo).

## Requisitos

- Go 1.22 o superior
- Una API key de Google Cloud con **YouTube Data API v3** habilitada

## Configuración

1. Copia `.env.example` a `.env` y pon tu API key:
   ```
   cp .env.example .env
   ```
2. Edita `.env` y completa `YOUTUBE_API_KEY`. Opcionalmente, agrega IDs de
  video en `BACKUP_PLAYLIST` (separados por coma) para que suenen cuando la
  cola esté vacía.
3. Configura `FRONTEND_BASE_URL` (por defecto `http://localhost:3000`) para
  que el backend pueda construir links `.../join/{sessionID}` y generar QR.

## Instalar dependencias y correr

```bash
go mod tidy
go run main.go
```

El servidor queda escuchando en `http://localhost:8080` (o el puerto que
pongas en `PORT`).

## Endpoints

### `GET /api/search?q=texto`
Busca canciones en YouTube. Devuelve solo videos embebibles, con duración
exacta ya resuelta.

```json
{
  "results": [
    {
      "videoId": "dQw4w9WgXcQ",
      "title": "...",
      "channel": "...",
      "thumbnail": "https://...",
      "durationSeconds": 212
    }
  ]
}
```

### `POST /api/sessions`
Crea una nueva sesión en memoria.

```json
// request
{ "name": "Cumpleaños de Ana" }
```

### `GET /api/sessions/{sessionID}`
Devuelve la sesión solicitada, incluyendo `joinUrl` para compartirla.

```json
{
  "id": "e2f6...",
  "name": "Cumpleaños de Ana",
  "createdAt": "2026-07-19T20:11:49Z",
  "status": "active",
  "joinUrl": "http://localhost:3000/join/e2f6..."
}
```

### `GET /api/sessions/{sessionID}/qr`
Devuelve un PNG (`Content-Type: image/png`) con un código QR que apunta al
`joinUrl` de la sesión (`{FRONTEND_BASE_URL}/join/{sessionID}`).

### `POST /api/sessions/{sessionID}/queue`
Agrega una canción a la cola de esa sesión. El backend vuelve a consultar
YouTube para título/duración reales — nunca confía en lo que mande el
cliente.

```json
// request
{ "videoId": "dQw4w9WgXcQ" }
```

### `GET /api/sessions/{sessionID}/queue`
Devuelve el estado completo de la cola de esa sesión: qué está sonando, la
cola pendiente y el tiempo estimado de espera (en segundos) para cada
elemento.

### `GET /api/sessions/{sessionID}/ws`
WebSocket. Recibe automáticamente un mensaje cada vez que el estado de la
cola de esa sesión cambia:
```json
{ "type": "queueState", "state": { "nowPlaying": {...}, "queue": [...], "estimatedWaitSecs": [...] } }
```

El **panel de reproducción** debe enviar este mensaje cuando el video termine
(evento `onStateChange` del IFrame Player con `YT.PlayerState.ENDED`):
```json
{ "type": "ended" }
```
Esto hace que el backend avance la cola automáticamente (siguiente pedido, o
el siguiente video de la playlist de respaldo si no hay pedidos).
