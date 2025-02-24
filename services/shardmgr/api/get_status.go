package api

type GetStatusResponse struct {
	Shards []ShardState `json:"shards"`
}

type ShardState struct {
	ShardId string `json:"shard_id"`
	// ShardState 是 shard 的状态
	ShardState string `json:"shard_state,omitempty"`
}
