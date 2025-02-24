# 获取版本信息
VERSION ?= $(shell cat VERSION)
GIT_COMMIT ?= $(shell git rev-parse --short HEAD)
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -X github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common.version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)

# Docker 镜像标签
DOCKER_TAG ?= $(VERSION)
DOCKER_IMAGE := xinkaiw/etcdmgr

# 所有命令行工具
CMDS := hello
SVCS := shardmgr

.PHONY: all fmt lint build test clean help docker-build docker-push $(CMDS) $(SVCS)

# 默认目标
all: fmt lint build test

# 构建所有组件
build: build-cmds build-svcs

# 构建服务
build-service:
	@echo "Building etcdmgr service..."
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/etcdmgr ./service/etcdmgr

# 构建命令行工具
build-cmds: $(CMDS)

# 为每个命令行工具创建目标
$(CMDS):
	@echo "Building $@..."
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/$@ ./cmd/$@

# 构建命令行工具
build-svcs: $(SVCS)

# 为每个命令行工具创建目标
$(SVCS):
	@echo "Building $@..."
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/$@ ./service/$@

# Docker 相关目标
docker-build:
	@echo "Building Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) -f Dockerfile .

docker-push:
	@echo "Pushing Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

# 运行测试
test:
	@echo "Running tests..."
	go test -v ./...

# 运行服务
run:
	@echo "Starting shardmgr service..."
	go run -ldflags "$(LDFLAGS)" service/shardmgr/shardmgr.go

# 代码格式化
fmt:
	@echo "Formatting code..."
	go fmt ./...
	gofmt -s -w .

# 代码检查
lint:
	@echo "Linting code..."
	go vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# 清理构建产物
clean:
	@echo "Cleaning build artifacts..."
	go clean
	rm -rf bin

# 帮助信息
help:
	@echo "Available targets:"
	@echo "  all          - Default target: fmt, lint, build, test"
	@echo "  build        - Build all components"
	@echo "  build-service - Build etcdmgr service"
	@echo "  build-cmds   - Build all command line tools"
	@echo "  test         - Run tests"
	@echo "  run          - Run the service"
	@echo "  fmt          - Format code"
	@echo "  lint         - Run linters"
	@echo "  clean        - Clean build artifacts"
	@echo "  help         - Show this help message"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-push  - Push Docker image"
	@echo ""
	@echo "Available commands:"
	@for cmd in $(CMDS); do \
		echo "  $$cmd         - Build $$cmd command"; \
	done
