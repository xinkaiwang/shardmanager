# Makefile for services/cougar

# 基本变量设置
# 如果有 VERSION 文件，使用它的内容，否则使用默认版本号
VERSION ?= v0.1.0
# 获取 Git 提交的短哈希值
GIT_COMMIT ?= $(shell git rev-parse --short HEAD)
# 获取当前构建时间
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')
# 定义构建标志
LDFLAGS := -X main.Version=$(VERSION) \
           -X main.GitCommit=$(GIT_COMMIT) \
           -X main.BuildTime=$(BUILD_TIME)

# 定义构建目录和输出目录
BIN_DIR := bin
MAIN_PKG := ./cmd/cougardemo
BINARY_NAME := cougardemo
BINARY_PATH := $(BIN_DIR)/$(BINARY_NAME)

# 定义 Go 命令
GO ?= go
GOFMT ?= gofmt
GOLINT ?= golangci-lint

# 默认目标，执行完整构建流程：fmt -> lint -> build -> test
all: fmt lint build test

# 创建必要的目录
$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

# 格式化代码
fmt:
	@echo "格式化代码..."
	@$(GOFMT) -s -w .

# 代码检查
lint:
	@echo "执行代码检查..."
	@if command -v $(GOLINT) > /dev/null; then \
		$(GOLINT) run; \
	else \
		echo "警告: golangci-lint 未安装，跳过代码检查"; \
		echo "可以使用 'go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest' 安装"; \
	fi

# 构建服务
build: $(BIN_DIR)
	@echo "构建 $(BINARY_NAME)..."
	@$(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY_PATH) $(MAIN_PKG)
	@echo "构建完成: $(BINARY_PATH)"

# 运行测试
test:
	@echo "运行测试..."
	@$(GO) test -v ./...

# Docker 构建（可选）
docker-build:
	@echo "构建 Docker 镜像..."
	@cd ../../ && docker build -t xinkaiw/$(BINARY_NAME):$(VERSION) -f services/cougar/Dockerfile .

# Docker 推送（可选）
docker-push:
	@echo "推送 Docker 镜像..."
	@docker push xinkaiw/$(BINARY_NAME):$(VERSION)

# 清理构建产物
clean:
	@echo "清理构建产物..."
	@rm -rf $(BIN_DIR)
	@$(GO) clean
	@echo "清理完成"

# 显示帮助信息
help:
	@echo "可用目标:"
	@echo "  all          - 执行完整的构建流程（fmt -> lint -> build -> test）"
	@echo "  fmt          - 格式化代码"
	@echo "  lint         - 代码检查"
	@echo "  build        - 构建服务"
	@echo "  test         - 运行测试"
	@echo "  docker-build - 构建 Docker 镜像"
	@echo "  docker-push  - 推送 Docker 镜像"
	@echo "  clean        - 清理构建产物"
	@echo "  help         - 显示帮助信息"

# 声明伪目标
.PHONY: all fmt lint build test docker-build docker-push clean help 