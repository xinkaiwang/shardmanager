# etcdmgr
This is a simple web UI tools to browse + maintain to etcd database.

# How to build
```
make build-etcdmgr
make build-ui
```

# How to run
```
export ETCD_ENDPOINTS=localhost:2379
./bin/etcdmgr
```

# start react ui
```
cd ui
npm install
npm run dev
```


这是一个 etcd 管理工具服务，主要功能包括：
Web UI 界面：
用于浏览和维护 etcd 数据库
提供可视化的 etcd 键值对管理界面
服务架构：
后端：Go 服务，提供 etcd 操作 API
前端：React UI，提供用户界面
使用 provider 模式抽象 etcd 操作

主界面：
1） 在网页输入要查询的path前缀， 显示从此path开始的N条数据
2） 一开始没有输入时从头“” 开始 查询
3） 数据可以很长，列表显示时需截断。
4） 用户可以从显示的列表中选择一条数据展开显示，这时可以显示全文。
5） 用户可以编辑选中的数据。
6） 用户可以删除选中的数据（编辑和删除功能不常用，需要考虑稍微藏起来一点，以免误触）。
7） 支持分页，分页本质上还是前缀搜索。所以URL要能支持编码当前搜索。
8） 用AJAX做前后端交互，要求用户体验要好。网页简洁，美观。考虑使用bootstrap这种成熟度高的套件。
配置管理：
通过环境变量配置 etcd 连接：
ETCD_ENDPOINTS：etcd 服务器地址
ETCD_DIAL_TIMEOUT：连接超时时间
构建和运行：
这是一个典型的全栈应用，提供了 etcd 的可视化管理功能。我们目前已经实现了 etcd provider 的核心功能，下一步应该是实现 HTTP API 和前端界面。

先思考，列出提纲 （在 Scratchpad 栏目），然后每一步都更新进度。

1) provider
2) make etcdmgr
