FROM golang:1.25 AS builder

# Set working directory
WORKDIR /app

# Pre-copy go.mod and go.sum for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary statically
RUN CGO_ENABLED=0 GOOS=linux go build -o /koorde ./cmd/node

# Use a minimal base image for the final stage
FROM gcr.io/distroless/base-debian12

# Copy binary and configuration
COPY --from=builder /koorde /usr/local/bin/koorde
COPY config/node/config.yaml /etc/koorde/config.yaml

ENTRYPOINT ["/usr/local/bin/koorde", "-config", "/etc/koorde/config.yaml"]
