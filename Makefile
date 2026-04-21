.PHONY: build test clean run lint docker-up docker-down

BINARY_NAME=agentx-proxy
GO=go
BUILD_DIR=bin

build:
	$(GO) build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/agentx-proxy

run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

test:
	$(GO) test -v -race -coverprofile=coverage.out ./...

test-coverage: test
	$(GO) tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

docker-up:
	docker compose -f docker/docker-compose.yml up -d

docker-down:
	docker compose -f docker/docker-compose.yml down

# Development
dev:
	$(GO) run ./cmd/agentx-proxy --config config.yaml

# Generate MySQL DDL from Prisma schema
generate-ddl:
	@echo "DDL files are pre-generated in internal/mysql/"

# Benchmark
bench:
	$(GO) test -bench=. -benchmem ./...
