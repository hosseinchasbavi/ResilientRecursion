.PHONY: build run test docker-build clean

# Build binary
build:
	go build -o bin/server ./cmd/server

# Run locally
run:
	go run ./cmd/server

# Run tests
test:
	go test -v ./...

# Build Docker image
docker-build:
	docker build -t sequence-calc:latest -f deployments/docker/Dockerfile .

# Clean build artifacts
clean:
	rm -rf bin/

# Install dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run