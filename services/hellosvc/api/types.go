package api

// HelloRequest 定义了 hello 接口的请求参数
type HelloRequest struct {
	Name string `json:"name"`
}

// HelloResponse 定义了 hello 接口的响应
type HelloResponse struct {
	Message string `json:"message"`
	Time    string `json:"time"`
}

// PingResponse 定义了 ping 接口的响应
type PingResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Version   string `json:"version"`
}
