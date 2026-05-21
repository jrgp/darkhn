# ── Stage 1: build ───────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

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

# Copy the statically-linked binary.
COPY --from=builder /bin/darkhn /darkhn

# Copy the CSS that is served at /inject.css.
COPY inject/ /inject/

EXPOSE 8080

ENV PORT=8080

ENTRYPOINT ["/darkhn"]
