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
