.PHONY: all build test clean run fmt lint help

# 默认目标
all: fmt lint build test

# 构建服务
build:
	@echo "Building hello service..."
	go build -v ./cmd/main.go

# 运行测试
test:
	@echo "Running tests..."
	go test -v ./...

# 运行服务
run:
	@echo "Starting hello service..."
	go run cmd/main.go

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
	rm -f main

# 帮助信息
help:
	@echo "Available targets:"
	@echo "  all        - Default target: fmt, lint, build, test"
	@echo "  build      - Build the service"
	@echo "  test       - Run tests"
	@echo "  run        - Run the service"
	@echo "  fmt        - Format code"
	@echo "  lint       - Run linters"
	@echo "  clean      - Clean build artifacts"
	@echo "  help       - Show this help message" 