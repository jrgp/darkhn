# ── Stage 1: build ───────────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Cache dependency downloads separately from the rest of the build.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build \
        -trimpath \
        -ldflags="-s -w" \
        -o /bin/darkhn \
        ./cmd/darkhn

# ── Stage 2: runtime ─────────────────────────────────────────────────────────
FROM scratch

# CA certificates are needed for TLS connections to the upstream HN site.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Copy the statically-linked binary.
COPY --from=builder /bin/darkhn /darkhn

# Copy the CSS that is served at /inject.css.
COPY inject/ /inject/

EXPOSE 8080

ENV PORT=8080

ENTRYPOINT ["/darkhn"]
