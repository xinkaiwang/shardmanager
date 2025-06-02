// 工作节点状态
export interface WorkerVm {
  worker_full_id: string;
  worker_id: string;
  session_id: string;
  is_offline: number;
  is_shutdown_req: number;
  is_draining: number;
  is_shutdown_permitted: number;
  assignments: AssignmentVm[];
}

// 分配状态
export interface AssignmentVm {
  assignment_id: string;
  shard_id: string;
  replica_idx: number;
  worker_full_id: string;
  status: string;
}

// 分片状态
export interface ShardVm {
  shard_id: string;
  replicas: ReplicaVm[];
}

// 副本状态
export interface ReplicaVm {
  replica_idx: number;
  assignments: string[];
}

// 获取状态请求
export interface GetStateRequest {
  favorite?: boolean;
  search?: string;
}

// 获取状态响应
export interface GetStateResponse {
  workers: WorkerVm[];
  shards: ShardVm[];
}

// API 错误响应类型
export interface ApiError {
  error: string;
  msg: string;
  code: string;
} 