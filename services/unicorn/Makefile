# 获取版本号
VERSION ?= $(shell cat VERSION 2>/dev/null || echo "v0.0.1")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD)
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Go 构建标志
LDFLAGS := -X main.Version=$(VERSION) \
           -X main.GitCommit=$(GIT_COMMIT) \
           -X main.BuildTime=$(BUILD_TIME)

# Docker 镜像配置
DOCKER_REPO := xinkaiw
IMAGE_NAME := unicorn
FULL_IMAGE_NAME := $(DOCKER_REPO)/$(IMAGE_NAME):$(VERSION)

.PHONY: all
all: fmt lint test

.PHONY: build
build:
	@echo "Building unicorn package..."
	go build ./...

.PHONY: test
test:
	@echo "Running tests..."
	go test -v ./...

.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...

.PHONY: lint
lint:
	@echo "Linting code..."
	go vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	go clean -cache

.PHONY: docker-build
docker-build:
	@echo "Building Docker image $(FULL_IMAGE_NAME)..."
	docker build -t $(FULL_IMAGE_NAME) \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-f Dockerfile ..

.PHONY: docker-push
docker-push:
	@echo "Pushing Docker image $(FULL_IMAGE_NAME)..."
	docker push $(FULL_IMAGE_NAME)

.PHONY: run
run: build
	@echo "Running unicorn service..."
	./bin/unicorn

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all            - Format, lint and test the code"
	@echo "  build          - Build the unicorn package"
	@echo "  test           - Run tests"
	@echo "  fmt            - Format Go code"
	@echo "  lint           - Run linters"
	@echo "  clean          - Clean build cache"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-push    - Push Docker image to registry"
	@echo "  run            - Build and run the service"
	@echo "  help           - Show this help message"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION        - Version number (default: v0.0.1)"
	@echo "  DOCKER_REPO    - Docker repository (default: xinkaiw)"
	@echo "  IMAGE_NAME     - Image name (default: unicorn)" 