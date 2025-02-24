package api

type GetStatusResponse struct {
	Shards  []ShardState  `json:"shards"`
	Workers []WorkerState `json:"workers"`
}

type ShardState struct {
	ShardId string `json:"shard_id"`
	// ShardState 是 shard 的状态
	ShardState string `json:"shard_state,omitempty"`
}

type WorkerState struct {
	WorkerFullId string `json:"worker_full_id"`
}
