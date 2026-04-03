package ratelimit

import (
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	maxTokens  = 5
	refillRate = 5.0 / 60.0 // tokens per second (5 per minute)
)

type bucket struct {
	tokens     float64
	lastRefill time.Time
}

// Limiter tracks per-IP token buckets. Each IP is allowed a burst of 5
// requests and refills at 5 requests per minute.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

// NewLimiter returns a Limiter ready for use.
func NewLimiter() *Limiter {
	return &Limiter{
		buckets: make(map[string]*bucket),
	}
}

// Allow returns true and consumes one token if the IP is within the rate limit.
// Returns false if the bucket is exhausted.
func (l *Limiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, ok := l.buckets[ip]
	if !ok {
		b = &bucket{tokens: maxTokens, lastRefill: now}
		l.buckets[ip] = b
	}

	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * refillRate
	if b.tokens > maxTokens {
		b.tokens = maxTokens
	}
	b.lastRefill = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Middleware returns an HTTP middleware that applies the Limiter per remote IP.
// Requests that exceed the limit receive 429 Too Many Requests.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if !l.Allow(ip) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
