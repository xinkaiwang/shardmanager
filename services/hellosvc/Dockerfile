# 构建阶段
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache git make

# 复制 go.mod 和 go.sum
COPY go.mod go.sum ./

# 复制整个项目代码
COPY . .

# 构建应用
RUN make build

# 使用轻量级基础镜像
FROM alpine:latest

# 安装运行时依赖和调试工具
RUN apk add --no-cache ca-certificates tzdata bash vim curl wget bind-tools tcpdump htop busybox-extras

# 设置 bash 为默认 shell
SHELL ["/bin/bash", "-c"]

# 设置时区
ENV TZ=Asia/Shanghai

# 创建非 root 用户
RUN adduser -D -u 1000 appuser

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/bin/hellosvc .

# 使用非 root 用户运行
USER appuser

# 暴露端口
EXPOSE 8080 9090

# 设置健康检查
HEALTHCHECK --interval=30s --timeout=3s \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/hello || exit 1

# 运行应用
CMD ["./hellosvc"] 