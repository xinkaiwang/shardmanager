package costfunc

type CostfuncCfg struct {
	ShardCountCostEnable bool
	ShardCountCostNorm   float64 // number of shards

	QpmCostEnable bool
	QpmCostNorm   float64 // qpm per worker

	CpuCostEnable bool
	CpuCostNorm   float64 // cpu usage per worker (1.0 means 1 core)

	MemCostEnable bool
	MemCostNorm   float64 // memory usage per worker (1.0 means 1GB)

	WorkerMaxAssignments int
}
