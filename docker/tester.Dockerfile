FROM golang:1.25 AS builder

WORKDIR /app

# Dependency caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build static binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /koorde-tester ./cmd/tester


# Prepare filesystem
FROM busybox:1.36 AS prep

# Create data directories (owned by root for this variant)
RUN mkdir -p /data/results && chown -R 0:0 /data


# Runtime (Distroless, root-enabled)
FROM gcr.io/distroless/base-debian12

# Copy writable dirs from prep stage
COPY --from=prep /data /data

# Copy binary and configuration
COPY --from=builder /koorde-tester /usr/local/bin/koorde
COPY config/tester/config.yaml /etc/koorde/config.yaml

# Run as root (needed for /var/run/docker.sock access)
USER 0:0

# Entrypoint
ENTRYPOINT ["/usr/local/bin/koorde", "-config", "/etc/koorde/config.yaml"]
