# DarkHN

Dark mode for Hacker News — a lightweight proxy that rewrites HN pages on the fly.

**To use: replace `news.ycombinator.com` in any URL with your DarkHN instance.**

---

## How it works

DarkHN is a Go HTTP proxy that fetches pages from Hacker News, transforms the HTML, and serves the result with a dark stylesheet injected. Each response is cached for 5 seconds so repeated loads don't hit HN redundantly.

Transformations applied to every page:

- All relative asset URLs (CSS, JS, images) are rewritten to absolute HN URLs
- Absolute HN links in anchors are converted to relative paths so navigation stays on your proxy
- `" Dark"` is appended to the page title
- Vote links, the submit button, hide/fave links, and the login button are removed
- Reply and comment-submission forms are replaced with links that open on the real HN site
- `inject.css` is appended to `<head>` to apply the dark theme

Bot traffic is rejected and per-IP rate limiting is enforced by middleware.

## Running locally

**Prerequisites:** Go 1.21+

```bash
# Build
make build

# Run (default port 8080)
make run

# Custom port
PORT=3000 ./bin/darkhn
```

Then open `http://localhost:8080` in your browser.

## Docker / Podman

Run directly from the registry (no build required):

```bash
# Docker
docker run -p 8080:8080 ghcr.io/jrgp/darkhn:latest

# Podman (rootless)
podman run --rm -p 8080:8080 ghcr.io/jrgp/darkhn:latest
```

Or build and run locally:

```bash
make docker-build
docker run -p 8080:8080 darkhn:latest
```

The final image is built `FROM scratch` — just the statically-linked binary and the `inject/` CSS directory. Images are published to `ghcr.io/jrgp/darkhn` on every push to `master` (`:latest`) and on version tags (`:v1.2.3`).

## Development

```bash
# Run all tests (with race detector)
make test

# Build binary only
make build
```

Tests live alongside the packages they cover:

| Package | What's tested |
|---|---|
| `internal/transform` | HTML transformation against `source.html` / `reference.html` golden fixtures |
| `internal/handler` | Proxy caching, error handling, HTML vs passthrough responses |
| `internal/middleware` | Rate limiting (per-IP isolation, burst, X-Forwarded-For) and bot protection |

## Project layout

```
cmd/darkhn/          main entry point
internal/
  transform/         HTML rewriting logic
  handler/           HTTP proxy handler + in-process cache
  middleware/        rate limiter and bot protection
inject/              dark-mode stylesheet served at /inject.css
source.html          sample upstream HN response (test input)
reference.html       expected transformed output (test reference)
Dockerfile           multi-stage build → scratch runtime image
Makefile             build / test / run / docker-build targets
```
