# 获取版本号
VERSION ?= $(shell cat ../VERSION)

# 默认目标
.PHONY: all
all: build

# 安装依赖
.PHONY: install
install:
	npm install

# 开发服务器
.PHONY: dev
dev:
	npm run dev

# 构建项目
.PHONY: build
build: install
	npm run build

# 运行测试
.PHONY: test
test: install
	npm run test

# 运行类型检查
.PHONY: type-check
type-check: install
	npm run type-check

# 运行 lint
.PHONY: lint
lint: install
	npm run lint

# 构建 Docker 镜像
.PHONY: docker-build
docker-build: build
	docker build -t xinkaiw/shardmgr-web:$(VERSION) .

# 推送 Docker 镜像
.PHONY: docker-push
docker-push: docker-build
	docker push xinkaiw/shardmgr-web:$(VERSION)

# 清理构建产物
.PHONY: clean
clean:
	rm -rf dist
	rm -rf node_modules

# 帮助信息
.PHONY: help
help:
	@echo "可用的目标："
	@echo "  install      - 安装依赖"
	@echo "  dev         - 启动开发服务器"
	@echo "  build       - 构建项目"
	@echo "  test        - 运行测试"
	@echo "  type-check  - 运行类型检查"
	@echo "  lint        - 运行代码检查"
	@echo "  docker-build - 构建 Docker 镜像"
	@echo "  docker-push  - 推送 Docker 镜像"
	@echo "  clean       - 清理构建产物"
	@echo "  help        - 显示帮助信息" 