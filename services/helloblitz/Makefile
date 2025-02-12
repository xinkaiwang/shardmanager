 # 获取版本信息
VERSION := $(shell cat VERSION)
GIT_COMMIT := $(shell git rev-parse --short HEAD)
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Go 构建标志
LDFLAGS := -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)
GOFLAGS := -ldflags "$(LDFLAGS)"

# Docker 相关变量
DOCKER_REPO := xinkaiw
DOCKER_IMAGE := helloblitz
DOCKER_TAG := $(VERSION)

.PHONY: all build test clean docker-build docker-push run

all: build

build:
	@echo "Building helloblitz..."
	@mkdir -p bin
	go build $(GOFLAGS) -o bin/helloblitz ./cmd

test:
	@echo "Running tests..."
	go test -v ./...

clean:
	@echo "Cleaning..."
	rm -rf bin/

docker-build:
	@echo "Building Docker image..."
	cd ../.. && docker build -t $(DOCKER_REPO)/$(DOCKER_IMAGE):$(DOCKER_TAG) -f services/helloblitz/Dockerfile .

docker-push:
	@echo "Pushing Docker image..."
	docker push $(DOCKER_REPO)/$(DOCKER_IMAGE):$(DOCKER_TAG)

run: build
	@echo "Running helloblitz..."
	./bin/helloblitz