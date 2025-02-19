# etcdmgr - etcd 管理器

etcdmgr 是一个用于管理和监控 etcd 键值对的 Web 应用程序。它提供了直观的用户界面和 RESTful API，使您能够轻松地查看、编辑和管理 etcd 中的数据。

## 功能特性

- 🌐 现代化的 Web 界面
  - 实时查看和编辑键值对
  - 支持按前缀搜索
  - 支持长文本值的编辑
  - 响应式设计，适配各种屏幕尺寸

- 🔄 RESTful API
  - 获取键值对列表
  - 读取单个键值
  - 设置键值
  - 删除键值
  - 服务状态检查
  - 服务健康检查

- 📊 监控集成
  - Prometheus 指标导出
  - 系统资源使用监控
  - 操作延迟统计

## 技术栈

### 后端
- Go 1.21+
- etcd v3 客户端
- OpenCensus (指标收集)
- Prometheus 导出器

### 前端
- React 18
- Material-UI v5
- TypeScript
- Vite

## 快速开始

### 本地开发

1. 启动本地 etcd：
```bash
# 确保 etcd 在运行并可访问
export ETCD_ENDPOINTS=localhost:2379
```

2. 构建并运行服务：
```bash
cd services/etcdmgr
make build
./bin/etcdmgr
```

3. 开发前端：
```bash
cd web
npm install
npm run dev
```

### Docker 部署

1. 构建镜像：
```bash
make docker-build
```

2. 运行容器：
```bash
docker run -d \
  -p 8080:8080 \
  -p 9090:9090 \
  -e ETCD_ENDPOINTS=etcd:2379 \
  xinkaiw/etcdmgr:latest
```

### Kubernetes 部署

1. 创建命名空间：
```bash
kubectl apply -f deploy/namespace.yaml
```

2. 部署服务：
```bash
kubectl apply -f deploy/etcdmgr-deploy.yaml
```

## 配置选项

环境变量：
- `ETCD_ENDPOINTS`: etcd 服务器地址，多个地址用逗号分隔（默认：localhost:2379）
- `ETCD_DIAL_TIMEOUT`: 连接超时时间（秒）（默认：5）
- `API_PORT`: HTTP API 端口（默认：8080）
- `METRICS_PORT`: Prometheus 指标端口（默认：9090）
- `LOG_LEVEL`: 日志级别（debug/info/warn/error）（默认：info）
- `LOG_FORMAT`: 日志格式（json/text）（默认：json）

## API 文档

### 健康检查
```http
GET /api/ping
```
返回服务健康状态和版本信息。

### 状态检查
```http
GET /api/status
```
返回服务状态信息。

### 列出键值对
```http
GET /api/list_keys?prefix={prefix}
```
列出指定前缀的所有键值对。

参数：
- `prefix`（可选）：键的前缀，用于过滤结果

响应示例：
```json
{
  "keys": [
    {
      "key": "my/key1",
      "value": "value1",
      "version": 123
    },
    {
      "key": "my/key2",
      "value": "value2",
      "version": 124
    }
  ]
}
```

### 获取单个键值
```http
GET /api/get_key?key={key}
```
获取指定键的值。

参数：
- `key`（必需）：要获取的键名

响应示例：
```json
{
  "key": "my/key1",
  "value": "value1",
  "version": 123
}
```

### 设置键值
```http
POST /api/set_key?key={key}
Content-Type: application/json

{
  "value": "新的值"
}
```
设置指定键的值。

参数：
- `key`（必需）：要设置的键名
- 请求体：包含新值的 JSON 对象

### 删除键值
```http
POST /api/delete_key?key={key}
```
删除指定的键。

参数：
- `key`（必需）：要删除的键名

## 监控

### Prometheus 指标

服务在 `/metrics` 端点（默认端口 9090）暴露以下指标：

- `etcdmgr_request_duration_seconds`: API 请求处理时间
- `etcdmgr_request_total`: API 请求总数
- `process_*`: 进程级别的系统指标

### 健康检查

服务提供了以下健康检查端点：
- `/api/ping`: 基本健康检查
- `/api/status`: 详细状态检查

## 开发指南

### 项目结构
```
etcdmgr/
├── api/          # API 类型定义
├── internal/     # 内部实现
│   ├── biz/     # 业务逻辑
│   ├── handler/ # HTTP 处理器
│   └── provider/# etcd 访问层
├── service/     # 服务入口
└── web/         # 前端代码
```

### 构建命令

- `make build`: 构建所有组件
- `make build-service`: 仅构建后端服务
- `make test`: 运行测试
- `make docker-build`: 构建 Docker 镜像
- `make docker-push`: 推送 Docker 镜像

## 许可证

[License 信息]