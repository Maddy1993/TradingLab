# TradingLab Makefile
# Variables
PROJECT_NAME := tradinglab
REGISTRY := localhost:5000
VERSION := $(shell git describe --tags --always --dirty || echo "dev")
NATS_VERSION := 2.9.15

# Go settings
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod
GOLINT := golangci-lint

# Docker settings
DOCKER := docker
DOCKER_BUILD := $(DOCKER) build

# Kubernetes settings
KUBECTL := kubectl
NAMESPACE := tradinglab

# Directories
GO_SRC_DIRS := ./cmd ./pkg ./internal
PYTHON_SRC_DIRS := ./strategy ./analysis

# Service names
API_GATEWAY_SERVICE := api-gateway
TRADINGLAB_SERVICE := tradinglab-service
TRADINGLAB_UI := tradinglab-ui
EVENT_CLIENT := event-client
MARKET_DATA_SERVICE := market-data-service
EVENT_HUB := event-hub

# Create bin directory if not exists
$(shell mkdir -p bin)

# Default target
.PHONY: all
all: setup build

# Setup development environment
.PHONY: setup
setup: setup-go setup-python

# Setup Go environment
.PHONY: setup-go
setup-go:
	@echo "Setting up Go environment..."
	$(GOMOD) tidy

# Setup Python environment
.PHONY: setup-python
setup-python:
	@echo "Setting up Python environment..."
	@if [ ! -d "venv" ]; then \
		python -m venv venv; \
		. venv/bin/activate && pip install --prefer-binary -r requirements.txt; \
	fi

# Build all services
.PHONY: build
build: build-go build-ui

# Build Go services
.PHONY: build-go
build-go: setup-go build-event-client build-market-data-service build-event-hub build-api-gateway


# Build Python services
.PHONY: build-python
build-python: build-tradinglab-service

# Build UI
.PHONY: build-ui
build-ui:
	@echo "Building UI..."
	@if [ -d "ui" ]; then \
		cd ui && npm install && npm run build; \
	else \
		echo "UI directory not found, skipping..."; \
	fi

# Build event client
.PHONY: build-event-client
build-event-client:
	@echo "Building event client..."
	@mkdir -p bin
	$(GOBUILD) -o bin/$(EVENT_CLIENT) ./cmd/event-client

# Build market data service
.PHONY: build-market-data-service
build-market-data-service:
	@echo "Building market data service..."
	@mkdir -p bin
	$(GOBUILD) -o bin/$(MARKET_DATA_SERVICE) ./cmd/market-data-service

# Build event hub
.PHONY: build-event-hub
build-event-hub:
	@echo "Building event hub..."
	@mkdir -p bin
	$(GOBUILD) -o bin/$(EVENT_HUB) ./cmd/event-hub

# Build API gateway (Go version)
.PHONY: build-api-gateway
build-api-gateway:
	@echo "Building Go API gateway..."
	@mkdir -p bin
	$(GOBUILD) -o bin/$(API_GATEWAY_SERVICE) ./cmd/gateway

# Build TradingLab service
.PHONY: build-tradinglab-service
build-tradinglab-service:
	@echo "Building TradingLab service..."
	@if [ -f "requirements.txt" ]; then \
		pip install -r requirements.txt; \
	fi

# Docker images
.PHONY: docker-images
docker-images: docker-event-client docker-market-data-service docker-event-hub docker-api-gateway docker-tradinglab-service docker-ui

# Build Go service images
.PHONY: docker-go-service-images
docker-go-service-images: docker-market-data-service docker-event-hub docker-api-gateway

.PHONY: build-deploy-go-services
build-deploy-go-services: docker-go-service-images docker-go-services-push deploy-go-services

# Docker image for event client
.PHONY: docker-event-client
docker-event-client:
	@echo "Building event client Docker image..."
	$(DOCKER_BUILD) --platform linux/amd64 -t $(REGISTRY)/$(EVENT_CLIENT):$(VERSION) -f docker/event-client/Dockerfile .
	$(DOCKER) tag $(REGISTRY)/$(EVENT_CLIENT):$(VERSION) $(REGISTRY)/$(EVENT_CLIENT):latest

# Docker image for market data service
.PHONY: docker-market-data-service
docker-market-data-service:
	@echo "Building market data service Docker image..."
	$(DOCKER_BUILD) --platform linux/amd64 -t $(REGISTRY)/$(MARKET_DATA_SERVICE):$(VERSION) -f docker/market-data/Dockerfile .
	$(DOCKER) tag $(REGISTRY)/$(MARKET_DATA_SERVICE):$(VERSION) $(REGISTRY)/$(MARKET_DATA_SERVICE):latest

# Docker image for event hub
.PHONY: docker-event-hub
docker-event-hub:
	@echo "Building event hub Docker image..."
	$(DOCKER_BUILD) --platform linux/amd64 -t $(REGISTRY)/$(EVENT_HUB):$(VERSION) -f docker/event-hub/Dockerfile .
	$(DOCKER) tag $(REGISTRY)/$(EVENT_HUB):$(VERSION) $(REGISTRY)/$(EVENT_HUB):latest

# Docker image for API gateway
.PHONY: docker-api-gateway
docker-api-gateway:
	@echo "Building API gateway Docker image..."
	$(DOCKER_BUILD) --platform linux/amd64 -t $(REGISTRY)/$(API_GATEWAY_SERVICE):$(VERSION) -f docker/gateway/Dockerfile .
	$(DOCKER) tag $(REGISTRY)/$(API_GATEWAY_SERVICE):$(VERSION) $(REGISTRY)/$(API_GATEWAY_SERVICE):latest

# Docker image for TradingLab service
.PHONY: docker-tradinglab-service
docker-tradinglab-service:
	@echo "Building TradingLab service Docker image..."
	$(DOCKER_BUILD) --platform linux/amd64 -t $(REGISTRY)/$(TRADINGLAB_SERVICE):$(VERSION) -f docker/server/Dockerfile .
	$(DOCKER) tag $(REGISTRY)/$(TRADINGLAB_SERVICE):$(VERSION) $(REGISTRY)/$(TRADINGLAB_SERVICE):latest

# Docker image for UI
.PHONY: docker-ui
docker-ui:
	@echo "Building UI Docker image..."
	$(DOCKER_BUILD) --platform linux/amd64 -t $(REGISTRY)/$(TRADINGLAB_UI):$(VERSION) -f docker/ui/Dockerfile .
	$(DOCKER) tag $(REGISTRY)/$(TRADINGLAB_UI):$(VERSION) $(REGISTRY)/$(TRADINGLAB_UI):latest

# Push Docker images
.PHONY: docker-push
docker-push:
	$(DOCKER) push $(REGISTRY)/$(EVENT_CLIENT):$(VERSION)
	$(DOCKER) push $(REGISTRY)/$(MARKET_DATA_SERVICE):$(VERSION)
	$(DOCKER) push $(REGISTRY)/$(EVENT_HUB):$(VERSION)
	$(DOCKER) push $(REGISTRY)/$(API_GATEWAY_SERVICE):$(VERSION)
	$(DOCKER) push $(REGISTRY)/$(TRADINGLAB_SERVICE):$(VERSION)
	$(DOCKER) push $(REGISTRY)/$(TRADINGLAB_UI):$(VERSION)

.PHONY: docker-go-services-push
docker-go-services-push:
	$(DOCKER) push $(REGISTRY)/$(MARKET_DATA_SERVICE):$(VERSION)
	$(DOCKER) push $(REGISTRY)/$(EVENT_HUB):$(VERSION)
	$(DOCKER) push $(REGISTRY)/$(API_GATEWAY_SERVICE):$(VERSION)

# Deploy to Kubernetes
.PHONY: deploy
deploy: deploy-nats deploy-services

# Deploy NATS
.PHONY: deploy-nats
deploy-nats:
	@echo "Deploying NATS..."
	$(KUBECTL) apply -f kube/nats/nats-deployment.yaml -n $(NAMESPACE)

# Deploy services
.PHONY: deploy-services
deploy-services:
	@echo "Deploying services..."
	$(eval export VERSION=$(shell git describe --tags --always --dirty || echo "dev"))
	$(eval export REGISTRY=$(REGISTRY))
	@echo "Using registry: $(REGISTRY) and version: $(VERSION)"
	$(KUBECTL) apply -f kube/event-client/event-client.yaml -n $(NAMESPACE)
	$(KUBECTL) apply -f kube/market-data/market-data.yaml -n $(NAMESPACE)
	$(KUBECTL) apply -f kube/event-hub/event-hub.yaml -n $(NAMESPACE)
	envsubst < kube/api-gateway/deployment.yaml | $(KUBECTL) apply -f - -n $(NAMESPACE)
	$(KUBECTL) apply -f kube/tradinglab/tradinglab-server.yaml -n $(NAMESPACE)
	$(KUBECTL) apply -f kube/tradinglab/tradinglab-lb-service.yaml -n $(NAMESPACE)
	envsubst < kube/ui/ui-deployment.yaml | $(KUBECTL) apply -f - -n $(NAMESPACE)
	if [ -f kube/ingress.yaml ]; then envsubst < kube/ingress.yaml | $(KUBECTL) apply -f - -n $(NAMESPACE); fi

# Deploy services
.PHONY: deploy-go-services
deploy-go-services:
	@echo "Deploying Go services..."
	$(eval export VERSION=$(shell git describe --tags --always --dirty || echo "dev"))
	$(eval export REGISTRY=$(REGISTRY))
	@echo "Using registry: $(REGISTRY) and version: $(VERSION)"
	envsubst < kube/market-data/market-data.yaml | $(KUBECTL) apply -f - -n $(NAMESPACE)
	envsubst < kube/event-hub/event-hub.yaml | $(KUBECTL) apply -f - -n $(NAMESPACE)
	envsubst < kube/api-gateway/deployment.yaml | $(KUBECTL) apply -f - -n $(NAMESPACE)

# Test
.PHONY: test
test: test-go test-python test-integration

# Test Go code
.PHONY: test-go
test-go:
	@echo "Testing Go code..."
	$(GOTEST) ./pkg/... ./cmd/... ./internal/...

# Test Python code
.PHONY: test-python
test-python:
	@echo "Testing Python code..."
	@if [ -d "tests" ]; then \
		python -m pytest tests/; \
	else \
		echo "Tests directory not found, skipping..."; \
	fi

# Integration tests
.PHONY: test-integration
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -tags=integration ./tests/integration/...

# Clean
.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -rf ui/build/
	find . -name "__pycache__" -type d -exec rm -rf {} +
	find . -name "*.pyc" -delete

# Help
.PHONY: help
help:
	@echo "TradingLab Makefile targets:"
	@echo "  all              - Setup and build all components"
	@echo "  setup            - Setup development environment"
	@echo "  build            - Build all services"
	@echo "  docker-images    - Build all Docker images"
	@echo "  docker-push      - Push all Docker images"
	@echo "  deploy           - Deploy to Kubernetes"
	@echo "  test             - Run tests"
	@echo "  clean            - Clean build artifacts"
	@echo "  help             - Show this help"