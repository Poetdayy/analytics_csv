# ── Stage 1: Builder ────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

# git needed by some go mod dependencies
RUN apk --no-cache add git ca-certificates

WORKDIR /app

# Copy dependency files first — Docker layer cache:
# only re-downloads modules when go.mod/go.sum change, not on every code edit
COPY go.mod go.sum* ./
RUN go mod download && go mod verify

# Copy source and build a fully static binary (CGO disabled)
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -trimpath -o ad-aggregator .

# ── Stage 2: Test runner (used in CI, skipped in prod build) ─────────────────
FROM builder AS tester
RUN apk --no-cache add build-base
RUN go test -race -v ./tests/

# ── Stage 3: Minimal runtime image (~7MB total) ──────────────────────────────
FROM scratch

# CA certificates for any outbound TLS (future-proofing)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy only the compiled binary — no OS, no shell, no attack surface
COPY --from=builder /app/ad-aggregator /ad-aggregator

VOLUME ["/data", "/results"]

ENTRYPOINT ["/ad-aggregator"]
CMD ["--input=/workspace/ad_data.csv", "--output=/results/"]
