FROM golang:1.25 AS builder

WORKDIR /app

# --- Dependency caching
COPY go.mod go.sum ./
RUN go mod download

# --- Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /koorde-node ./cmd/node

FROM gcr.io/distroless/base-debian12

# --- Copy binary and configuration
COPY --from=builder /koorde-node /usr/local/bin/koorde
COPY config/node/config.yaml /etc/koorde/config.yaml

# --- Entry point
ENTRYPOINT ["/usr/local/bin/koorde", "-config", "/etc/koorde/config.yaml"]
