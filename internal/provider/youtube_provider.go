package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

const youtubeBaseURL = "https://www.googleapis.com/youtube/v3"

// YouTubeProvider encapsula las llamadas a YouTube Data API v3.
type YouTubeProvider struct {
	apiKey     string
	httpClient *http.Client
}

func NewYouTubeProvider(apiKey string) *YouTubeProvider {
	return &YouTubeProvider{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 8 * time.Second,
		},
	}
}

// --- Respuestas crudas de la API de YouTube ---

type youtubeSearchResponse struct {
	Items []struct {
		ID struct {
			VideoID string `json:"videoId"`
		} `json:"id"`
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

type youtubeVideosResponse struct {
	Items []struct {
		ID             string `json:"id"`
		ContentDetails struct {
			Duration string `json:"duration"`
		} `json:"contentDetails"`
		Status struct {
			Embeddable bool `json:"embeddable"`
		} `json:"status"`
	} `json:"items"`
}

// Search busca videos por texto libre y devuelve resultados normalizados.
func (p *YouTubeProvider) Search(query string) ([]Track, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("%w: missing API key", ErrProviderUnavailable)
	}
	if query == "" {
		return nil, ErrInvalidQuery
	}

	searchURL := fmt.Sprintf(
		"%s/search?part=snippet&type=video&videoEmbeddable=true&maxResults=10&q=%s&key=%s",
		youtubeBaseURL,
		url.QueryEscape(query),
		p.apiKey,
	)

	var sr youtubeSearchResponse
	if err := p.getJSON(searchURL, &sr); err != nil {
		return nil, err
	}

	if len(sr.Items) == 0 {
		return []Track{}, nil
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

	details, err := p.videoDetails(ids)
	if err != nil {
		return nil, err
	}

	tracks := make([]Track, 0, len(details))
	for _, d := range details {
		if !d.embeddable {
			continue
		}
		m := meta[d.id]
		tracks = append(tracks, Track{
			ID:              d.id,
			Title:           m.Title,
			Channel:         m.Channel,
			Thumbnail:       m.Thumbnail,
			DurationSeconds: d.durationSeconds,
		})
	}

	return tracks, nil
}

// GetDetails resuelve una canción por ID, reconsultando el proveedor.
func (p *YouTubeProvider) GetDetails(trackID string) (*Track, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("%w: missing API key", ErrProviderUnavailable)
	}
	if trackID == "" {
		return nil, ErrInvalidTrackID
	}

	sr, err := p.searchTitleFallback(trackID)
	if err != nil {
		return nil, err
	}

	details, err := p.videoDetails([]string{trackID})
	if err != nil {
		return nil, err
	}
	if len(details) == 0 {
		return nil, ErrTrackNotFound
	}
	if !details[0].embeddable {
		return nil, ErrTrackUnavailable
	}

	return &Track{
		ID:              trackID,
		Title:           sr.title,
		Channel:         sr.channel,
		Thumbnail:       sr.thumbnail,
		DurationSeconds: details[0].durationSeconds,
	}, nil
}

type youtubeVideoDetail struct {
	id              string
	durationSeconds int
	embeddable      bool
}

func (p *YouTubeProvider) videoDetails(ids []string) ([]youtubeVideoDetail, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	idsParam := ids[0]
	for _, id := range ids[1:] {
		idsParam += "," + id
	}

	videosURL := fmt.Sprintf(
		"%s/videos?part=contentDetails,status&id=%s&key=%s",
		youtubeBaseURL,
		url.QueryEscape(idsParam),
		p.apiKey,
	)

	var vr youtubeVideosResponse
	if err := p.getJSON(videosURL, &vr); err != nil {
		return nil, err
	}

	out := make([]youtubeVideoDetail, 0, len(vr.Items))
	for _, item := range vr.Items {
		out = append(out, youtubeVideoDetail{
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

func (p *YouTubeProvider) searchTitleFallback(trackID string) (*titleInfo, error) {
	snippetURL := fmt.Sprintf(
		"%s/videos?part=snippet&id=%s&key=%s",
		youtubeBaseURL,
		url.QueryEscape(trackID),
		p.apiKey,
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

	if err := p.getJSON(snippetURL, &raw); err != nil {
		return nil, err
	}
	if len(raw.Items) == 0 {
		return nil, ErrTrackNotFound
	}

	s := raw.Items[0].Snippet
	return &titleInfo{title: s.Title, channel: s.ChannelTitle, thumbnail: s.Thumbnails.Medium.URL}, nil
}

func (p *YouTubeProvider) getJSON(reqURL string, out interface{}) error {
	resp, err := p.httpClient.Get(reqURL)
	if err != nil {
		return fmt.Errorf("%w: network error", ErrProviderUnavailable)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return ErrTrackNotFound
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError {
			return fmt.Errorf("%w: upstream status %d", ErrProviderUnavailable, resp.StatusCode)
		}
		return fmt.Errorf("%w: upstream status %d", ErrProviderUnavailable, resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("%w: invalid upstream response", ErrProviderUnavailable)
	}
	return nil
}

var iso8601DurationRegex = regexp.MustCompile(`PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?`)

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
