FROM golang:1.25 AS builder

WORKDIR /app

# --- Dependency caching
COPY go.mod go.sum ./
RUN go mod download

# --- Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /koorde-client ./cmd/client

# Runtime image
FROM gcr.io/distroless/base-debian12

WORKDIR /app

# --- Copy binary
COPY --from=builder /koorde-client /usr/local/bin/koorde-client

# --- Default to help if no args are provided
ENTRYPOINT ["/usr/local/bin/koorde-client"]
CMD ["--help"]
