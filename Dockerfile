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

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o removarr ./cmd/removarr

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/removarr .

# Copy example config
COPY --from=builder /app/config.example.yaml ./config.example.yaml

# Expose port
EXPOSE 8080

# Run the application
CMD ["./removarr"]

