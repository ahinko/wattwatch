.PHONY: build run test test-handlers test-repos test-all migrate-up migrate-down swagger provider-run provider-run-manual docker

# Build the application
build:
	go build -o bin/api cmd/api/main.go

# Run the application
run:
	go run cmd/api/main.go

# Run all tests (handlers first, then repositories)
test-all: test-handlers test-repos

# Run only handler tests
test-handlers:
	go test -v ./internal/api/handlers/...

# Run only repository tests
test-repos:
	go test -v ./internal/repository/...

# Default test command now runs handlers first, then repos
test: test-all

# Generate swagger documentation
swagger:
	swag init -g cmd/api/main.go -o docs

# Run a provider once (e.g., make provider-run PROVIDER=nordpool)
provider-run:
	@if [ -z "$(PROVIDER)" ]; then \
		echo "Error: PROVIDER is required. Usage: make provider-run PROVIDER=nordpool"; \
		exit 1; \
	fi
	go run cmd/provider/main.go --provider $(PROVIDER) --once

# Run a provider manually with specific options
# Usage: make provider-run-manual PROVIDER=nordpool DATE=2024-03-20 ZONE=SE1 CURRENCY=EUR
provider-run-manual:
	@if [ -z "$(PROVIDER)" ] || [ -z "$(DATE)" ] || [ -z "$(ZONE)" ] || [ -z "$(CURRENCY)" ]; then \
		echo "Error: All parameters are required."; \
		echo "Usage: make provider-run-manual PROVIDER=nordpool DATE=2024-03-20 ZONE=SE1 CURRENCY=EUR"; \
		exit 1; \
	fi
	go run cmd/provider/main.go --provider $(PROVIDER) --once \
		--date $(DATE) --zone $(ZONE) --currency $(CURRENCY)

# Docker image name and tag
DOCKER_IMAGE := wattwatch
DOCKER_TAG := $(shell git rev-parse --short HEAD)

.PHONY: docker
docker:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
