# 构建阶段
FROM golang:1.21-alpine AS builder

# 安装依赖
RUN apk add --no-cache git ca-certificates tzdata

# 设置工作目录
WORKDIR /app

# 复制 Go 模块文件
COPY go.mod go.sum ./
COPY services/cougar/go.mod services/cougar/go.sum ./services/cougar/

# 下载依赖
RUN go mod download

# 复制源码
COPY libs ./libs/
COPY services/cougar ./services/cougar/

# 构建应用
RUN cd services/cougar && \
    VERSION=$(cat VERSION) && \
    GO111MODULE=on CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags "-X main.Version=$VERSION -X main.GitCommit=$(git rev-parse --short HEAD) -X main.BuildTime=$(date -u '+%Y-%m-%d_%H:%M:%S')" \
    -o /go/bin/cougardemo ./cmd/cougardemo

# 最终阶段
FROM alpine:3.19

# 安装必要的运行时依赖
RUN apk add --no-cache ca-certificates tzdata

# 从构建阶段复制二进制文件
COPY --from=builder /go/bin/cougardemo /usr/local/bin/

# 设置时区
ENV TZ=America/Los_Angeles

# 非 root 用户运行
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

# 设置入口点
ENTRYPOINT ["cougardemo"] 