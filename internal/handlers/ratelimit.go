package handlers

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	maxRequests = 20
	window      = 10 * time.Second
)

type bucket struct {
	count      int
	windowFrom time.Time
}

// IPRateLimiter implementa un límite de solicitudes por IP con ventana fija.
type IPRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

func NewIPRateLimiter() *IPRateLimiter {
	l := &IPRateLimiter{buckets: make(map[string]*bucket)}

	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			l.mu.Lock()
			for ip, b := range l.buckets {
				if time.Since(b.windowFrom) > window {
					delete(l.buckets, ip)
				}
			}
			l.mu.Unlock()
		}
	}()

	return l
}

func (l *IPRateLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[ip]
	if !ok || time.Since(b.windowFrom) > window {
		l.buckets[ip] = &bucket{count: 1, windowFrom: time.Now()}
		return true
	}

	b.count++
	return b.count <= maxRequests
}

// clientIP toma la IP original de X-Forwarded-For (Render no expone la IP real en RemoteAddr).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first := strings.TrimSpace(strings.Split(xff, ",")[0])
		if first != "" {
			return first
		}
	}
	return r.RemoteAddr
}
