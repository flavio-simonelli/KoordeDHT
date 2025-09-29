FROM golang:1.25 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download


COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /koorde ./cmd/node


FROM gcr.io/distroless/base-debian12

COPY --from=builder /koorde /usr/local/bin/koorde
COPY config/node/config.yaml /etc/koorde/config.yaml

ENTRYPOINT ["/usr/local/bin/koorde"]
