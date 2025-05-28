# 构建阶段
FROM golang:1.23-alpine AS builder

WORKDIR /src/shardmanager

# 安装构建依赖
RUN apk add --no-cache git make

# 复制依赖库
COPY libs/xklib ./libs/xklib/
COPY libs/cougar ./libs/cougar/
COPY libs/unicorn ./libs/unicorn/

# 复制服务代码
COPY services/shardmgr ./services/shardmgr/

# 复制 go.mod 和 go.sum
COPY go.mod go.sum ./

# 设置构建参数
ARG VERSION
ARG GIT_COMMIT
ARG BUILD_TIME

# 进入服务目录构建
WORKDIR /src/shardmanager/services/shardmgr
RUN if [ -z "$VERSION" ]; then VERSION=$(cat VERSION); fi && \
    if [ -z "$GIT_COMMIT" ]; then GIT_COMMIT="unknown"; fi && \
    if [ -z "$BUILD_TIME" ]; then BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S'); fi && \
    make build-svcs LDFLAGS="-X github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common.version=${VERSION} -X main.GitCommit=${GIT_COMMIT} -X main.BuildTime=${BUILD_TIME}"

# 最终镜像
FROM alpine:3.18

# 安装运行时依赖和调试工具
RUN apk add --no-cache ca-certificates tzdata bash curl wget bind-tools

# 设置时区
ENV TZ=America/Los_Angeles

# 创建非root用户
RUN adduser -D -h /app appuser

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /src/shardmanager/services/shardmgr/bin/shardmgr /app/shardmgr

# 添加元数据
LABEL maintainer="xinkaiwang"
LABEL description="ShardManager service"
LABEL version="${VERSION}"

# 切换到非root用户
USER appuser

# 设置健康检查
HEALTHCHECK --interval=30s --timeout=3s \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/ping || exit 1

# 设置入口点
ENTRYPOINT ["/app/shardmgr"]

# 默认命令（可被覆盖）
CMD [] 