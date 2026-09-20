package provider

import (
	"strings"
	"sync"
	"time"
)

const (
	defaultCacheTTL      = 10 * time.Minute
	defaultCleanupPeriod = 5 * time.Minute
)

type cacheEntry struct {
	tracks  []Track
	savedAt time.Time
}

// CachedProvider wraps a provider and adds an in-memory search cache.
type CachedProvider struct {
	next      MusicProvider
	cache     map[string]cacheEntry
	cacheLock sync.RWMutex
	ttl       time.Duration
}

func NewCachedProvider(next MusicProvider) *CachedProvider {
	p := &CachedProvider{
		next:  next,
		cache: make(map[string]cacheEntry),
		ttl:   defaultCacheTTL,
	}

	go func() {
		ticker := time.NewTicker(defaultCleanupPeriod)
		defer ticker.Stop()

		for range ticker.C {
			p.cacheLock.Lock()
			for key, entry := range p.cache {
				if time.Since(entry.savedAt) > p.ttl {
					delete(p.cache, key)
				}
			}
			p.cacheLock.Unlock()
		}
	}()

	return p
}

func (p *CachedProvider) Search(query string) ([]Track, error) {
	normalizedKey := strings.TrimSpace(strings.ToLower(query))
	if normalizedKey == "" {
		return p.next.Search(query)
	}

	p.cacheLock.RLock()
	entry, ok := p.cache[normalizedKey]
	p.cacheLock.RUnlock()
	if ok && time.Since(entry.savedAt) <= p.ttl {
		return cloneTracks(entry.tracks), nil
	}

	tracks, err := p.next.Search(query)
	if err != nil {
		return nil, err
	}

	p.cacheLock.Lock()
	p.cache[normalizedKey] = cacheEntry{
		tracks:  cloneTracks(tracks),
		savedAt: time.Now(),
	}
	p.cacheLock.Unlock()

	return cloneTracks(tracks), nil
}

func (p *CachedProvider) GetDetails(trackID string) (*Track, error) {
	return p.next.GetDetails(trackID)
}

func cloneTracks(tracks []Track) []Track {
	if len(tracks) == 0 {
		return nil
	}

	cloned := make([]Track, len(tracks))
	copy(cloned, tracks)
	return cloned
}
