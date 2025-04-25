# 第一阶段：构建React应用
FROM node:18-alpine AS build

WORKDIR /app

# 复制依赖文件
COPY package.json package-lock.json ./

# 安装依赖
RUN npm ci

# 复制其余源代码
COPY . .

# 构建应用
RUN npm run build

# 第二阶段：使用Nginx提供服务
FROM nginx:alpine

# 安装gettext工具包，提供envsubst命令
RUN apk add --no-cache gettext

# 从构建阶段复制构建产物
COPY --from=build /app/dist /usr/share/nginx/html

# 复制入口脚本和Nginx配置模板
COPY docker-entrypoint.sh /
COPY nginx.conf.template /etc/nginx/templates/nginx.conf.template

# 设置入口脚本可执行权限
RUN chmod +x /docker-entrypoint.sh

# 设置环境变量默认值
ENV API_URL=http://localhost:8080

# 暴露80端口
EXPOSE 80

# 设置入口点
ENTRYPOINT ["/docker-entrypoint.sh"] 