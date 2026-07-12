package youtube

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"musica-colaborativa-api/internal/models"
)

const baseURL = "https://www.googleapis.com/youtube/v3"

// Client encapsula las llamadas a la YouTube Data API v3.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 8 * time.Second,
		},
	}
}

// --- Respuestas crudas de la API de YouTube ---

type searchResponse struct {
	Items []struct {
		ID struct {
			VideoID string `json:"videoId"`
		} `json:"id"`
		Snippet struct {
			Title      string `json:"title"`
			ChannelTitle string `json:"channelTitle"`
			Thumbnails struct {
				Medium struct {
					URL string `json:"url"`
				} `json:"medium"`
			} `json:"thumbnails"`
		} `json:"snippet"`
	} `json:"items"`
}

type videosResponse struct {
	Items []struct {
		ID             string `json:"id"`
		ContentDetails struct {
			Duration string `json:"duration"` // formato ISO 8601, ej: PT3M25S
		} `json:"contentDetails"`
		Status struct {
			Embeddable bool `json:"embeddable"`
		} `json:"status"`
	} `json:"items"`
}

// Search busca canciones/videos por texto libre y devuelve solo los que son
// embebibles, con su duración exacta ya resuelta.
func (c *Client) Search(query string) ([]models.Song, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("YOUTUBE_API_KEY no está configurada")
	}

	searchURL := fmt.Sprintf(
		"%s/search?part=snippet&type=video&videoEmbeddable=true&maxResults=10&q=%s&key=%s",
		baseURL, url.QueryEscape(query), c.apiKey,
	)

	var sr searchResponse
	if err := c.getJSON(searchURL, &sr); err != nil {
		return nil, fmt.Errorf("buscando en YouTube: %w", err)
	}

	if len(sr.Items) == 0 {
		return []models.Song{}, nil
	}

	ids := make([]string, 0, len(sr.Items))
	meta := map[string]struct {
		Title, Channel, Thumbnail string
	}{}
	for _, item := range sr.Items {
		id := item.ID.VideoID
		if id == "" {
			continue
		}
		ids = append(ids, id)
		meta[id] = struct{ Title, Channel, Thumbnail string }{
			Title:     item.Snippet.Title,
			Channel:   item.Snippet.ChannelTitle,
			Thumbnail: item.Snippet.Thumbnails.Medium.URL,
		}
	}

	details, err := c.videoDetails(ids)
	if err != nil {
		return nil, fmt.Errorf("obteniendo duración de videos: %w", err)
	}

	songs := make([]models.Song, 0, len(details))
	for _, d := range details {
		if !d.embeddable {
			continue // respetamos videos que el dueño bloqueó para reproducción externa
		}
		m := meta[d.id]
		songs = append(songs, models.Song{
			VideoID:         d.id,
			Title:           m.Title,
			Channel:         m.Channel,
			Thumbnail:       m.Thumbnail,
			DurationSeconds: d.durationSeconds,
		})
	}

	return songs, nil
}

// GetVideoDetails resuelve una sola canción por su videoID, re-consultando
// la API (nunca confiamos en datos que mande el cliente al agregar a la cola).
func (c *Client) GetVideoDetails(videoID string) (*models.Song, error) {
	sr, err := c.searchTitleFallback(videoID)
	if err != nil {
		return nil, err
	}

	details, err := c.videoDetails([]string{videoID})
	if err != nil {
		return nil, err
	}
	if len(details) == 0 || !details[0].embeddable {
		return nil, fmt.Errorf("el video no está disponible o no es embebible")
	}

	return &models.Song{
		VideoID:         videoID,
		Title:           sr.title,
		Channel:         sr.channel,
		Thumbnail:       sr.thumbnail,
		DurationSeconds: details[0].durationSeconds,
	}, nil
}

type videoDetail struct {
	id              string
	durationSeconds int
	embeddable      bool
}

func (c *Client) videoDetails(ids []string) ([]videoDetail, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	idsParam := ids[0]
	for _, id := range ids[1:] {
		idsParam += "," + id
	}

	videosURL := fmt.Sprintf(
		"%s/videos?part=contentDetails,status&id=%s&key=%s",
		baseURL, url.QueryEscape(idsParam), c.apiKey,
	)

	var vr videosResponse
	if err := c.getJSON(videosURL, &vr); err != nil {
		return nil, err
	}

	out := make([]videoDetail, 0, len(vr.Items))
	for _, item := range vr.Items {
		out = append(out, videoDetail{
			id:              item.ID,
			durationSeconds: parseISO8601Duration(item.ContentDetails.Duration),
			embeddable:      item.Status.Embeddable,
		})
	}
	return out, nil
}

type titleInfo struct {
	title, channel, thumbnail string
}

// searchTitleFallback usa el endpoint videos.list con part=snippet para traer
// título/canal/miniatura de un video puntual (más barato que buscar por texto).
func (c *Client) searchTitleFallback(videoID string) (*titleInfo, error) {
	snippetURL := fmt.Sprintf(
		"%s/videos?part=snippet&id=%s&key=%s",
		baseURL, url.QueryEscape(videoID), c.apiKey,
	)

	var raw struct {
		Items []struct {
			Snippet struct {
				Title        string `json:"title"`
				ChannelTitle string `json:"channelTitle"`
				Thumbnails   struct {
					Medium struct {
						URL string `json:"url"`
					} `json:"medium"`
				} `json:"thumbnails"`
			} `json:"snippet"`
		} `json:"items"`
	}

	if err := c.getJSON(snippetURL, &raw); err != nil {
		return nil, err
	}
	if len(raw.Items) == 0 {
		return nil, fmt.Errorf("video no encontrado: %s", videoID)
	}

	s := raw.Items[0].Snippet
	return &titleInfo{title: s.Title, channel: s.ChannelTitle, thumbnail: s.Thumbnails.Medium.URL}, nil
}

func (c *Client) getJSON(reqURL string, out interface{}) error {
	resp, err := c.httpClient.Get(reqURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("YouTube API respondió con status %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

var iso8601DurationRegex = regexp.MustCompile(`PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?`)

// parseISO8601Duration convierte "PT3M25S" -> 205 (segundos).
func parseISO8601Duration(d string) int {
	matches := iso8601DurationRegex.FindStringSubmatch(d)
	if matches == nil {
		return 0
	}
	hours, _ := strconv.Atoi(matches[1])
	minutes, _ := strconv.Atoi(matches[2])
	seconds, _ := strconv.Atoi(matches[3])
	return hours*3600 + minutes*60 + seconds
}
