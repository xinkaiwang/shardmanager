// API 响应类型定义
export interface StatusResponse {
  status: string;
  version: string;
  timestamp: string;
}

export interface EtcdKeyResponse {
  key: string;
  value: string;
  version: number;
}

export interface EtcdKeysResponse {
  keys: EtcdKeyResponse[];
}

export interface EtcdKeyRequest {
  value: string;
}

// API 错误响应类型
export interface ApiError {
  error: string;
  msg: string;
  code: string;
} 