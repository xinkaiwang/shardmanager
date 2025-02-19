package api

// EtcdKeyRequest 定义了 etcd key 操作的请求参数
type EtcdKeyRequest struct {
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

// EtcdKeyResponse 定义了 etcd key 操作的响应
type EtcdKeyResponse struct {
	Key     string `json:"key"`
	Value   string `json:"value,omitempty"`
	Version int64  `json:"version,omitempty"`
}

// EtcdKeysResponse 定义了获取多个 key 的响应
type EtcdKeysResponse struct {
	Keys []EtcdKeyResponse `json:"keys"`
}

// StatusResponse 定义了服务状态响应
type StatusResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}

// PingResponse 定义了 ping 接口的响应
type PingResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Version   string `json:"version"`
}
