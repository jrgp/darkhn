// Package middleware provides HTTP middleware for the DarkHN proxy.
package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ipEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// ipRateLimiter manages per-IP token-bucket rate limiters.
type ipRateLimiter struct {
	mu    sync.Mutex
	ips   map[string]*ipEntry
	r     rate.Limit
	burst int
}

func newIPRateLimiter(r rate.Limit, burst int) *ipRateLimiter {
	l := &ipRateLimiter{
		ips:   make(map[string]*ipEntry),
		r:     r,
		burst: burst,
	}
	go l.cleanup()
	return l
}

func (l *ipRateLimiter) get(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.ips[ip]
	if !ok {
		e = &ipEntry{limiter: rate.NewLimiter(l.r, l.burst)}
		l.ips[ip] = e
	}
	e.lastSeen = time.Now()
	return e.limiter
}

// cleanup removes limiters that haven't been seen in 10 minutes.
func (l *ipRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		cutoff := time.Now().Add(-10 * time.Minute)
		l.mu.Lock()
		for ip, e := range l.ips {
			if e.lastSeen.Before(cutoff) {
				delete(l.ips, ip)
			}
		}
		l.mu.Unlock()
	}
}

// RateLimit returns middleware that allows at most r requests/second per IP
// with the given burst size. It respects X-Forwarded-For when present so that
// clients behind a trusted reverse proxy are limited by their real IP.
func RateLimit(r float64, burst int) func(http.Handler) http.Handler {
	lim := newIPRateLimiter(rate.Limit(r), burst)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ip := realIP(req)
			if !lim.get(ip).Allow() {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}

// realIP extracts the client IP from the request, using X-Forwarded-For if set.
func realIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the leftmost (client) address.
		parts := strings.SplitN(xff, ",", 2)
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
