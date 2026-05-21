package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func okHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// ---- BotProtection tests ----

func TestBotProtection_AllowsBrowserUA(t *testing.T) {
	h := BotProtection(http.HandlerFunc(okHandler))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestBotProtection_BlocksEmptyUA(t *testing.T) {
	h := BotProtection(http.HandlerFunc(okHandler))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestBotProtection_BlocksKnownBot(t *testing.T) {
	bots := []string{
		"Scrapy/2.5",
		"python-requests/2.28",
		"AhrefsBot/7.0",
		"SemrushBot/7~bl",
		"Go-http-client/2.0",
	}
	h := BotProtection(http.HandlerFunc(okHandler))
	for _, ua := range bots {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("User-Agent", ua)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusForbidden, rr.Code, "UA %q should be blocked", ua)
	}
}

// ---- RateLimit tests ----

func TestRateLimit_AllowsUnderLimit(t *testing.T) {
	h := RateLimit(100, 10)(http.HandlerFunc(okHandler))
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "request %d should be allowed", i)
	}
}

func TestRateLimit_BlocksOverBurst(t *testing.T) {
	// Burst of 2: first two requests allowed, third should be rejected.
	h := RateLimit(0.0001, 2)(http.HandlerFunc(okHandler))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "5.6.7.8:9999"
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "request %d should pass", i)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "5.6.7.8:9999"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code, "request over burst should be blocked")
}

func TestRateLimit_IsolatesIPs(t *testing.T) {
	// Burst of 1 per IP; different IPs should not share quota.
	h := RateLimit(0.0001, 1)(http.HandlerFunc(okHandler))

	for _, ip := range []string{"10.0.0.1:1", "10.0.0.2:1", "10.0.0.3:1"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "first request from %s should pass", ip)
	}
}

func TestRateLimit_RespectsXForwardedFor(t *testing.T) {
	h := RateLimit(0.0001, 1)(http.HandlerFunc(okHandler))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.42, 10.0.0.1")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// Same real IP again – should be rate-limited.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "127.0.0.1:5678"
	req2.Header.Set("X-Forwarded-For", "203.0.113.42, 10.0.0.2")
	rr2 := httptest.NewRecorder()
	h.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusTooManyRequests, rr2.Code)
}
