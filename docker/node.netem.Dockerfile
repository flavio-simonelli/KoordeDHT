FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /koorde-node ./cmd/node

# Debian with tc installed
FROM debian:12-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends iproute2 ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /koorde-node /usr/local/bin/koorde
COPY config/node/config.yaml /etc/koorde/config.yaml

ENTRYPOINT ["/usr/local/bin/koorde", "-config", "/etc/koorde/config.yaml"]
