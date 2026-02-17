.PHONY: proto clean build test docker-up docker-down

# Generate all proto files
proto:
	@echo "Generating proto files..."
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/auth/*.proto
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/user/*.proto
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/content/*.proto
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/ai/*.proto
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/quiz/*.proto
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/progress/*.proto
	@echo "Proto generation complete!"

# Build all services
build:
	@echo "Building all services..."
	cd services/auth && go build -o ../../bin/auth cmd/main.go
	cd services/user && go build -o ../../bin/user cmd/main.go
	cd services/content && go build -o ../../bin/content cmd/main.go
	cd services/ai && go build -o ../../bin/ai cmd/main.go
	cd services/quiz && go build -o ../../bin/quiz cmd/main.go
	cd services/progress && go build -o ../../bin/progress cmd/main.go
	cd services/notification && go build -o ../../bin/notification cmd/main.go
	@echo "Build complete!"

# Run tests
test:
	@echo "Running tests..."
	go test -v ./services/...
	@echo "Tests complete!"

# Start all services with Docker
docker-up:
	docker-compose up --build -d

# Stop all services
docker-down:
	docker-compose down -v

# Clean generated files
clean:
	rm -rf bin/
	find proto -name "*.pb.go" -delete
	@echo "Clean complete!"

# Install Go dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run ./...
