# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install dependencies for building
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application and migration tool
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o removarr ./cmd/removarr && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o migrate ./cmd/migrate

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata postgresql-client

WORKDIR /root/

# Copy binaries from builder
COPY --from=builder /app/removarr .
COPY --from=builder /app/migrate .

# Copy example config and migrations
COPY --from=builder /app/config.example.yaml ./config.example.yaml
COPY --from=builder /app/migrations ./migrations

# Expose port
EXPOSE 8080

# Run the application
CMD ["./removarr"]

