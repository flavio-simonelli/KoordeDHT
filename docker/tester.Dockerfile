# ---------- Stage 1: Build ----------
FROM golang:1.25 AS builder

WORKDIR /app

# --- Dependency caching
COPY go.mod go.sum ./
RUN go mod download

# --- Copy source and build static binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /koorde-tester ./cmd/tester


# ---------- Stage 2: Prepare filesystem ----------
FROM busybox:1.36 AS prep

# --- Create data directories with correct ownership for distroless runtime ---
RUN mkdir -p /data/results && chown -R 65532:65532 /data


# ---------- Stage 3: Runtime (Distroless) ----------
FROM gcr.io/distroless/base-debian12

# --- Copy pre-initialised writable dirs from prep stage ---
COPY --from=prep /data /data

# --- Copy binary and configuration ---
COPY --from=builder /koorde-tester /usr/local/bin/koorde
COPY config/tester/config.yaml /etc/koorde/config.yaml

# --- Run as non-root (nobody:nogroup) ---
USER 65532:65532

# --- Entrypoint ---
ENTRYPOINT ["/usr/local/bin/koorde", "-config", "/etc/koorde/config.yaml"]
