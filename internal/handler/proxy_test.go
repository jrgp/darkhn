package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClient is an HTTPClient that returns a canned response.
type fakeClient struct {
	statusCode  int
	body        string
	contentType string
}

func (f *fakeClient) Do(r *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Type", f.contentType)
	rec.WriteHeader(f.statusCode)
	io.WriteString(rec, f.body)
	return rec.Result(), nil
}

const minimalHTML = `<html><head><title>Hacker News</title></head><body>
<table><tr><td><span class="pagetop">
  <a href="login?goto=news">login</a>
</span></td></tr></table>
</body></html>`

func newTestProxy(client HTTPClient) *Proxy {
	return newProxyWithClient("https://news.ycombinator.com", client)
}

func TestProxy_HTMLResponse_IsTransformed(t *testing.T) {
	p := newTestProxy(&fakeClient{
		statusCode:  200,
		body:        minimalHTML,
		contentType: "text/html; charset=utf-8",
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	p.Handle(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "text/html")
	body := rr.Body.String()
	assert.Contains(t, body, "Hacker News Dark", "title should have Dark suffix")
	assert.Contains(t, body, "inject.css", "inject.css should be present")
	assert.NotContains(t, body, `href="login?goto=news"`, "login link should be gone")
}

func TestProxy_NonHTMLResponse_PassThrough(t *testing.T) {
	cssBody := "body { color: red; }"
	p := newTestProxy(&fakeClient{
		statusCode:  200,
		body:        cssBody,
		contentType: "text/css",
	})

	req := httptest.NewRequest(http.MethodGet, "/news.css", nil)
	rr := httptest.NewRecorder()
	p.Handle(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, cssBody, rr.Body.String())
}

func TestProxy_404_ReturnsUnknown(t *testing.T) {
	p := newTestProxy(&fakeClient{
		statusCode:  404,
		body:        "",
		contentType: "text/plain",
	})

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rr := httptest.NewRecorder()
	p.Handle(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Equal(t, "Unknown.", strings.TrimSpace(rr.Body.String()))
}

func TestProxy_500_ReturnsError(t *testing.T) {
	p := newTestProxy(&fakeClient{
		statusCode:  500,
		body:        "",
		contentType: "text/plain",
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	p.Handle(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestProxy_Cache_ServesSecondRequestFromCache(t *testing.T) {
	callCount := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, minimalHTML)
	}))
	defer upstream.Close()

	p := newProxyWithClient(upstream.URL, upstream.Client())

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()
		p.Handle(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
	}

	assert.Equal(t, 1, callCount, "upstream should only be called once; subsequent requests served from cache")
}

func TestIsHTML(t *testing.T) {
	assert.True(t, isHTML("text/html"))
	assert.True(t, isHTML("text/html; charset=utf-8"))
	assert.False(t, isHTML("text/css"))
	assert.False(t, isHTML("application/json"))
	assert.False(t, isHTML(""))
}
