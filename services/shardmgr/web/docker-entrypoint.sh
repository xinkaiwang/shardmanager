#!/bin/sh
set -e

# 默认API URL，如果未设置则使用本地地址
: "${API_URL:=http://localhost:8080}"

echo "Using API_URL: $API_URL"

# 将环境变量替换到nginx配置模板中
envsubst '${API_URL}' < /etc/nginx/templates/nginx.conf.template > /etc/nginx/conf.d/default.conf

# 启动nginx
exec nginx -g 'daemon off;' 