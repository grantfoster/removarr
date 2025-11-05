.PHONY: build run test clean migrate-up migrate-down migrate-reset migrate-version sqlc generate swagger

# Build the application
build:
	go build -o removarr ./cmd/removarr

# Run the application
run:
	go run ./cmd/removarr

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -f removarr
	go clean

# Run database migrations up
migrate-up:
	go run ./cmd/migrate -config config.yaml -cmd up

# Run database migrations down
migrate-down:
	go run ./cmd/migrate -config config.yaml -cmd down

# Reset database (drop all and re-run migrations)
migrate-reset:
	go run ./cmd/migrate -config config.yaml -cmd reset

# Check migration version
migrate-version:
	go run ./cmd/migrate -config config.yaml -cmd version

# Generate sqlc code
sqlc:
	sqlc generate

# Generate swagger docs
swagger:
	swag init -g cmd/removarr/main.go -o docs

# Generate sqlc and swagger
generate: sqlc swagger

# Install dependencies
deps:
	go mod download
	go mod tidy
