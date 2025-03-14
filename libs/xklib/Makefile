SHELL=/bin/bash

# 版本信息
VERSION ?= $(shell cat VERSION 2>/dev/null || echo "v0.1.0")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -X main.Version=$(VERSION) \
           -X main.GitCommit=$(GIT_COMMIT) \
           -X main.BuildTime=$(BUILD_TIME)

GO_BIN ?= go
BUILD_DIR = build
PKGS = ./...

.PHONY: all build test test-race lint fmt clean help

# 默认目标
all: fmt lint build test

# 构建目标
build: 
	mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 $(GO_BIN) build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/hello ./cmd/hello

# 测试目标
test:
	$(GO_BIN) test -timeout 30s $(PKGS)

# 带 race 检测的测试
test-race:
	$(GO_BIN) test -race -timeout 60s $(PKGS)

# 代码格式化
fmt:
	$(GO_BIN) fmt $(PKGS)

# 代码检查
lint:
	$(GO_BIN) vet $(PKGS)
	@echo "Checking for unformatted files..."
	@if [ -n "$$($(GO_BIN) fmt $(PKGS))" ]; then \
		echo "Please run 'make fmt' to format your code."; \
		exit 1; \
	fi

# 清理构建产物
clean:
	rm -rf $(BUILD_DIR)
	$(GO_BIN) clean

# 模块整理
tidy:
	$(GO_BIN) mod tidy

# 创建版本文件
version:
	@echo "$(VERSION)" > VERSION
	@echo "Version: $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"

# 帮助信息
help:
	@echo "Available targets:"
	@echo "  all        - 运行 fmt、lint、build 和 test 目标（默认）"
	@echo "  build      - 构建 cmd/hello 命令"
	@echo "  test       - 运行测试"
	@echo "  test-race  - 运行带 race 检测的测试"
	@echo "  fmt        - 格式化代码"
	@echo "  lint       - 检查代码风格"
	@echo "  clean      - 清理构建产物"
	@echo "  tidy       - 整理 Go 模块依赖"
	@echo "  version    - 创建或更新版本文件"
	@echo "  help       - 显示此帮助信息"