# --- Build stage (identico) --------------------------------------------------
FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /koorde-node ./cmd/node

# --- Runtime stage: Debian with tc ------------------------------------------
# Distroless è troppo minimale per test di rete: usiamo Debian base
FROM debian:12-slim

# Installiamo solo tc e le dipendenze essenziali
RUN apt-get update && \
    apt-get install -y --no-install-recommends iproute2 ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Copia del binario e config
COPY --from=builder /koorde-node /usr/local/bin/koorde
COPY config/node/config.yaml /etc/koorde/config.yaml

ENTRYPOINT ["/usr/local/bin/koorde", "-config", "/etc/koorde/config.yaml"]
