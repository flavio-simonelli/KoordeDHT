# ---------- Fase di build ----------
FROM golang:1.25 AS builder

WORKDIR /app

# Copia go.mod e go.sum per la cache
COPY ../go.mod go.sum ./
RUN go mod download

# Copia tutto il codice sorgente
COPY .. .

# Compila il tester
RUN go build -o koorde-tester ./cmd/tester

# ---------- Fase di runtime ----------
FROM gcr.io/distroless/base-debian12

WORKDIR /app

# Copia il binario dal builder
COPY --from=builder /app/koorde-tester .
COPY config/tester/config.yaml /etc/koorde/config.yaml

# Default: mostra help
ENTRYPOINT ["./koorde-tester"]
