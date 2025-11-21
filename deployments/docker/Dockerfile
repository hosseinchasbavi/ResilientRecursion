# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /bin/server \
    ./cmd/server

# Final stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates

COPY --from=builder /bin/server /bin/server

EXPOSE 8080

ENV GOGC=50
ENV GOMAXPROCS=1

ENTRYPOINT ["/bin/server"]