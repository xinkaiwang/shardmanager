# 获取版本信息
VERSION ?= $(shell cat VERSION)
GIT_COMMIT ?= $(shell git rev-parse --short HEAD)
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -X github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common.version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)

# Docker 镜像标签
DOCKER_TAG ?= $(VERSION)
DOCKER_IMAGE := xinkaiw/shardmgr

# 所有命令行工具
CMDS := hello
SVCS := shardmgr

# 测试日志目录
TEST_LOGS_DIR := ./logs
TEST_LOG_FILE := $(TEST_LOGS_DIR)/test-$(shell date +%Y%m%d-%H%M%S).log

.PHONY: all fmt lint build test test-with-log test-verbose clean help docker-build docker-push $(CMDS) $(SVCS)

# 默认目标
all: fmt lint build test-with-log

# 构建所有组件
build: build-cmds build-svcs

# 构建服务
build-service:
	@echo "Building shardmgr service..."
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/shardmgr ./service/shardmgr

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
	cd ../../ && docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		-f services/shardmgr/Dockerfile .

docker-push:
	@echo "Pushing Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)

# 创建日志目录
$(TEST_LOGS_DIR):
	@mkdir -p $(TEST_LOGS_DIR)

# 运行测试
test:
	@echo "Running tests..."
	@echo "注意：测试可能显示数据竞争警告，主要源于底层库 krunloop 中的采样器和RunLoop之间的并发访问。"
	@echo "这些并发问题需要在 krunloop 包中通过适当的同步机制解决，而不是忽略竞争检测。"
	go test -v ./...

# 运行测试并记录日志
test-with-log: $(TEST_LOGS_DIR)
	@echo "Running tests with logging to $(TEST_LOG_FILE)..."
	@echo "注意：测试可能显示数据竞争警告，主要源于底层库 krunloop 中的采样器和RunLoop之间的并发访问。" | tee -a $(TEST_LOG_FILE)
	@echo "这些并发问题需要在 krunloop 包中通过适当的同步机制解决，而不是忽略竞争检测。" | tee -a $(TEST_LOG_FILE)
	@echo "==== 测试开始：$(shell date) ====" | tee -a $(TEST_LOG_FILE)
	@go test -v ./... 2>&1 | tee -a $(TEST_LOG_FILE)
	@echo "==== 测试结束：$(shell date) ====" | tee -a $(TEST_LOG_FILE)
	@echo "测试日志已保存到: $(TEST_LOG_FILE)"
	@echo "查看测试失败: grep -E 'FAIL:|panic:' $(TEST_LOG_FILE)"

# 运行详细测试并记录日志（适用于调试间歇性失败）
test-verbose: $(TEST_LOGS_DIR)
	@echo "Running verbose tests with logging to $(TEST_LOG_FILE)..."
	@echo "==== 详细测试开始：$(shell date) ====" | tee -a $(TEST_LOG_FILE)
	@go test -v -count=1 -race ./... 2>&1 | tee -a $(TEST_LOG_FILE)
	@echo "==== 详细测试结束：$(shell date) ====" | tee -a $(TEST_LOG_FILE)
	@echo "详细测试日志已保存到: $(TEST_LOG_FILE)"
	@echo "查看测试失败摘要: grep -A 3 -B 3 -E 'FAIL:|panic:' $(TEST_LOG_FILE)"

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
	@echo "  build-service - Build shardmgr service"
	@echo "  build-cmds   - Build all command line tools"
	@echo "  test         - Run tests"
	@echo "  test-with-log - Run tests and save output to log file"
	@echo "  test-verbose - Run tests with race detection and detailed output to log file"
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
