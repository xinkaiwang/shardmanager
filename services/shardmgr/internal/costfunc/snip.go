package costfunc

type ShardSnip struct {
	Cost *Cost
}

func NewShardSnip() *ShardSnip {
	return &ShardSnip{}
}

type WorkerSnip struct {
	Cost *Cost
}

func NewWorkerSnip() *WorkerSnip {
	return &WorkerSnip{}
}
