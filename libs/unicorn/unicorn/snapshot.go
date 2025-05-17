package unicorn

import (
	"github.com/xinkaiwang/shardmanager/libs/unicorn/data"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/unicornjson"
)

type Snapshot struct {
	AllWorkers map[data.WorkerId]*WorkerSnap
	AllShards  map[data.ShardId]*ShardSnap
}

type WorkerSnap struct {
	WorkerId data.WorkerId
}

func NewWorkerSnap(workerEntry *unicornjson.WorkerEntryJson) *WorkerSnap {
	return &WorkerSnap{
		WorkerId: data.WorkerId(workerEntry.WorkerId),
	}
}

type ShardSnap struct {
	ShardId  data.ShardId
	Replicas map[data.ReplicaIdx]*ReplicaSnap
}

func NewShardSnap(shardId data.ShardId) *ShardSnap {
	return &ShardSnap{
		ShardId:  shardId,
		Replicas: make(map[data.ReplicaIdx]*ReplicaSnap),
	}
}

type ReplicaSnap struct {
	ReplicaIdx data.ReplicaIdx
	WorkerId   data.WorkerId
}
