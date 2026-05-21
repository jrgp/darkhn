// Package handler implements the HTTP proxy handler for DarkHN.
package handler

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/jrgp/darkhn/internal/transform"
)

const (
	cacheTTL  = 5 * time.Second
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/81.0.4044.138 Safari/537.36"
)

// HTTPClient is the interface satisfied by *http.Client, allowing injection of
// a test double.
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type cacheEntry struct {
	body        []byte
	contentType string
	statusCode  int
	expiry      time.Time
}

// Proxy fetches pages from the upstream HN site, transforms their HTML,
// and serves the result with a short in-process cache.
type Proxy struct {
	upstream  string // e.g. "https://news.ycombinator.com"
	client    HTTPClient
	mu        sync.RWMutex
	cache     map[string]*cacheEntry
}

// NewProxy creates a production-ready Proxy targeting the real HN site.
func NewProxy() *Proxy {
	p := &Proxy{
		upstream: transform.MirrorProtocol + "://" + transform.MirrorHost,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		cache: make(map[string]*cacheEntry),
	}
	go p.sweepCache()
	return p
}

// newProxyWithClient creates a Proxy with a custom HTTP client and upstream,
// intended for testing.
func newProxyWithClient(upstream string, client HTTPClient) *Proxy {
	p := &Proxy{
		upstream: upstream,
		client:   client,
		cache:    make(map[string]*cacheEntry),
	}
	return p
}

func (p *Proxy) sweepCache() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		p.mu.Lock()
		for k, e := range p.cache {
			if now.After(e.expiry) {
				delete(p.cache, k)
			}
		}
		p.mu.Unlock()
	}
}

func (p *Proxy) fromCache(key string) (*cacheEntry, bool) {
	p.mu.RLock()
	e, ok := p.cache[key]
	p.mu.RUnlock()
	if !ok || time.Now().After(e.expiry) {
		return nil, false
	}
	return e, true
}

func (p *Proxy) store(key string, e *cacheEntry) {
	p.mu.Lock()
	p.cache[key] = e
	p.mu.Unlock()
}

// Handle is the chi-compatible handler for all proxied GET requests.
func (p *Proxy) Handle(w http.ResponseWriter, r *http.Request) {
	mirrorURL := p.upstream + r.RequestURI
	log.Printf("Request sent for %s", mirrorURL)

	if cached, ok := p.fromCache(mirrorURL); ok {
		w.Header().Set("Content-Type", cached.contentType)
		w.WriteHeader(cached.statusCode)
		w.Write(cached.body) //nolint:errcheck
		return
	}

	upstream, err := http.NewRequestWithContext(r.Context(), http.MethodGet, mirrorURL, nil)
	if err != nil {
		http.Error(w, "DarkHN server error.", http.StatusInternalServerError)
		return
	}
	upstream.Header.Set("User-Agent", userAgent)

	resp, err := p.client.Do(upstream)
	if err != nil {
		log.Printf("upstream error for %s: %v", mirrorURL, err)
		http.Error(w, "DarkHN server error.", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(resp.StatusCode)
		if resp.StatusCode == http.StatusNotFound {
			fmt.Fprint(w, "Unknown.")
		} else {
			fmt.Fprintf(w, "Error %d", resp.StatusCode)
		}
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "DarkHN server error.", http.StatusInternalServerError)
		return
	}

	ct := resp.Header.Get("Content-Type")
	var outBody []byte
	var outCT string

	if isHTML(ct) {
		transformed, err := transform.HTML(bytes.NewReader(body), mirrorURL)
		if err != nil {
			log.Printf("transform error for %s: %v", mirrorURL, err)
			http.Error(w, "DarkHN server error.", http.StatusInternalServerError)
			return
		}
		outBody = []byte(transformed)
		outCT = "text/html; charset=utf-8"
	} else {
		outBody = body
		outCT = ct
	}

	p.store(mirrorURL, &cacheEntry{
		body:        outBody,
		contentType: outCT,
		statusCode:  http.StatusOK,
		expiry:      time.Now().Add(cacheTTL),
	})

	w.Header().Set("Content-Type", outCT)
	w.Write(outBody) //nolint:errcheck
}

func isHTML(ct string) bool {
	return len(ct) >= 9 && ct[:9] == "text/html"
}
