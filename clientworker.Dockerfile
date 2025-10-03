FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o koorde-cli-worker ./cmd/client-worker

FROM gcr.io/distroless/base-debian12

WORKDIR /app

COPY --from=builder /app/koorde-cli-worker .

ENTRYPOINT ["/app/koorde-client"]
