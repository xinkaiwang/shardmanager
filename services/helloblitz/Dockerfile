# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache git make

# 复制源代码
COPY libs/xklib ./libs/xklib/
COPY services/helloblitz ./services/helloblitz/
COPY go.mod go.sum ./

# 构建应用
WORKDIR /app/services/helloblitz
RUN make build

# Final stage
FROM alpine:3.19

# 安装调试工具
RUN apk add --no-cache \
    bash \
    ca-certificates \
    tzdata \
    curl \
    wget \
    busybox-extras \
    tcpdump \
    bind-tools \
    netcat-openbsd \
    strace \
    htop \
    vim

# 设置 bash 为默认 shell
SHELL ["/bin/bash", "-c"]

# 设置时区
ENV TZ=Asia/Shanghai

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/services/helloblitz/bin/helloblitz /app/

# 设置默认环境变量
ENV BLITZ_THREAD_COUNT=3 \
    BLITZ_LOOP_SLEEP_MS=0 \
    BLITZ_TARGET_URL=http://localhost:8080/api/ping

ENTRYPOINT ["/app/helloblitz"]