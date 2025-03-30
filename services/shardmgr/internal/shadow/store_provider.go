package shadow

import (
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/smgjson"
)

type StoreProvider interface {
	// StoreShardState: store shard state to etcd
	StoreShardState(shardId data.ShardId, shardState *smgjson.ShardStateJson)

	// StoreWorkerState: store worker state to etcd
	StoreWorkerState(workerFullId data.WorkerFullId, workerState *smgjson.WorkerStateJson)

	Visit(visitor func(shadowState *ShadowState))
}
