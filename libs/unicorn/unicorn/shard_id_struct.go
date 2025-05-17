package unicorn

type ShardIdStruct interface {
	// return true if this key is in this shard
	KeyInShard(shardingKey uint32) bool
}
