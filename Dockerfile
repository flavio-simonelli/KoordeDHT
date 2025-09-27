FROM golang:1.25 AS builder
LABEL authors="flaviosimonelli"

WORKDIR /app

# Copia tutto il codice dentro il container
COPY . .

# Compila il binario
RUN go build -o /koorde ./cmd/node/

# Fase finale: immagine leggera
FROM debian:12-slim

WORKDIR /app

# Copia il binario compilato
COPY --from=builder /koorde /app/node

# Porta gRPC (modifica se diversa)
EXPOSE 41781

ENTRYPOINT ["/app/node"]
