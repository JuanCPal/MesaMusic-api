package provider

import (
	"errors"
	"testing"
	"time"
)

type stubSearchProvider struct {
	calls int
	res   []Track
	err   error
}

func (s *stubSearchProvider) Search(query string) ([]Track, error) {
	s.calls++
	return s.res, s.err
}

func (s *stubSearchProvider) GetDetails(trackID string) (*Track, error) {
	return &Track{ID: trackID}, nil
}

func TestCachedProviderCachesSuccessfulSearchesByNormalizedQuery(t *testing.T) {
	next := &stubSearchProvider{res: []Track{{ID: "1", Title: "Heart of Glass"}}}
	cached := NewCachedProvider(next)

	first, err := cached.Search(" Blondie ")
	if err != nil {
		t.Fatalf("first search returned error: %v", err)
	}
	if len(first) != 1 || first[0].ID != "1" {
		t.Fatalf("unexpected first result: %#v", first)
	}

	second, err := cached.Search("blondie")
	if err != nil {
		t.Fatalf("second search returned error: %v", err)
	}
	if len(second) != 1 || second[0].ID != "1" {
		t.Fatalf("unexpected second result: %#v", second)
	}
	if next.calls != 1 {
		t.Fatalf("expected one upstream call, got %d", next.calls)
	}
}

func TestCachedProviderDoesNotCacheErrorsOrExpiredEntries(t *testing.T) {
	next := &stubSearchProvider{err: errors.New("upstream failure")}
	cached := NewCachedProvider(next)

	if _, err := cached.Search("tina turner"); err == nil {
		t.Fatal("expected upstream error to be returned")
	}
	if next.calls != 1 {
		t.Fatalf("expected first failed search to reach upstream once, got %d", next.calls)
	}
	if _, err := cached.Search("tina turner"); err == nil {
		t.Fatal("expected second call to retry after failed cache attempt")
	}
	if next.calls != 2 {
		t.Fatalf("expected retry after unsuccessful result, got %d calls", next.calls)
	}

	next.err = nil
	next.res = []Track{{ID: "2", Title: "Simply the Best"}}
	if _, err := cached.Search("simply the best"); err != nil {
		t.Fatalf("expected successful search to be cached: %v", err)
	}
	if next.calls != 3 {
		t.Fatalf("expected one upstream call for first successful search, got %d", next.calls)
	}

	cached.cacheLock.Lock()
	cached.cache["simply the best"] = cacheEntry{tracks: []Track{{ID: "2", Title: "Simply the Best"}}, savedAt: time.Now().Add(-cached.ttl - time.Second)}
	cached.cacheLock.Unlock()

	if _, err := cached.Search("SIMPLY THE BEST"); err != nil {
		t.Fatalf("expected expired cache to refresh: %v", err)
	}
	if next.calls != 4 {
		t.Fatalf("expected expired entry to trigger a refresh, got %d calls", next.calls)
	}
}
