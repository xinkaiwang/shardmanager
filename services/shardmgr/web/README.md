# ShardManager Web UI

ShardManager 的 Web 管理界面，提供工作节点状态监控和配置管理功能。

## 功能

- 工作节点状态监控：查看所有工作节点及其分配的任务状态
- 配置管理：查看和修改系统配置（即将推出）

## 开发

### 前提条件

- Node.js 18+
- npm 8+

### 安装依赖

```bash
npm install
```

### 本地开发

```bash
npm run dev
```

这将启动开发服务器，默认地址为 http://localhost:5173。

API 请求会被代理到 http://localhost:8080，确保 ShardManager 服务在该地址运行。

### 构建项目

```bash
npm run build
```

构建产物将输出到 `dist` 目录。

## Docker 部署

### 构建 Docker 镜像

```bash
docker build -t shardmgr-ui .
```

### 运行 Docker 容器

```bash
docker run -d -p 8081:80 -e API_URL=http://your-api-host:8080 --name shardmgr-ui shardmgr-ui
```

访问 http://localhost:8081 即可使用 Web UI。

## 环境变量

| 变量名 | 描述 | 默认值 |
|-------|------|-------|
| API_URL | 后端 API 地址 | http://localhost:8080 | 