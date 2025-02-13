# 获取版本信息
VERSION ?= $(shell cat VERSION)
GIT_COMMIT ?= $(shell git rev-parse --short HEAD)
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)

# 所有命令行工具
CMDS := hello

.PHONY: all fmt lint build test clean help $(CMDS)

# 默认目标
all: fmt lint build test

# 构建所有命令行工具
build: $(CMDS)

# 为每个命令行工具创建目标
$(CMDS):
	@echo "Building $@..."
	@mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/$@ ./cmd/$@

# 运行测试
test:
	@echo "Running tests..."
	go test -v ./...

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
	@echo "  all        - Default target: fmt, lint, build, test"
	@echo "  build      - Build all commands"
	@echo "  test       - Run tests"
	@echo "  fmt        - Format code"
	@echo "  lint       - Run linters"
	@echo "  clean      - Clean build artifacts"
	@echo "  help       - Show this help message"
	@echo ""
	@echo "Available commands:"
	@for cmd in $(CMDS); do \
		echo "  $$cmd       - Build $$cmd command"; \
	done
