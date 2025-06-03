package core

import (
	"context"
	"sync/atomic"
)

// for metrics collection use only
type MetricsValues struct {
	// for metrics collection use only
	MetricsValueDynamicThreshold        atomic.Int64
	MetricsValueWorkerCount_total       atomic.Int64
	MetricsValueWorkerCount_online      atomic.Int64
	MetricsValueWorkerCount_offline     atomic.Int64
	MetricsValueWorkerCount_shutdownReq atomic.Int64
	MetricsValueWorkerCount_draining    atomic.Int64
	MetricsValueShardCount              atomic.Int64
	MetricsValueReplicaCount            atomic.Int64
	MetricsValueAssignmentCount         atomic.Int64
	// soft/hard score for current and future snapshots
	MetricsValueCurrentSoftCost atomic.Int64 // soft cost for current snapshot
	MetricsValueCurrentHardCost atomic.Int64 // hard cost for current snapshot
	MetricsValueFutureSoftCost  atomic.Int64 // soft cost for future snapshot
	MetricsValueFutureHardCost  atomic.Int64 // hard cost for future snapshot

}

func (ss *ServiceState) collectWorkerStats(ctx context.Context) {
	var workerCountTotal int64
	var workerCountShutdownReq int64
	var workerCountDraining int64
	var workerCountOffline int64
	var workerCountOnline int64

	for _, workerState := range ss.AllWorkers {
		// metrics
		workerCountTotal++
		if workerState.IsOnline() {
			workerCountOnline++
		}
		if workerState.IsOffline() {
			workerCountOffline++
		}
		if workerState.ShutdownRequesting {
			workerCountShutdownReq++
		}
		if workerState.HasShutdownHat() {
			workerCountDraining++
		}
	}

	ss.MetricsValues.MetricsValueWorkerCount_total.Store(workerCountTotal)
	ss.MetricsValues.MetricsValueWorkerCount_online.Store(workerCountOnline)
	ss.MetricsValues.MetricsValueWorkerCount_offline.Store(workerCountOffline)
	ss.MetricsValues.MetricsValueWorkerCount_draining.Store(workerCountDraining)
	ss.MetricsValues.MetricsValueWorkerCount_shutdownReq.Store(workerCountShutdownReq)
}

func (ss *ServiceState) collectShardStats(ctx context.Context) {
	shardCountTotal := int64(len(ss.AllShards))
	var replicaCountTotal int64
	assignmentCount := int64(len(ss.AllAssignments))

	for _, shardState := range ss.AllShards {
		replicaCountTotal += int64(len(shardState.Replicas))
	}
	ss.MetricsValues.MetricsValueShardCount.Store(shardCountTotal)
	ss.MetricsValues.MetricsValueReplicaCount.Store(replicaCountTotal)
	ss.MetricsValues.MetricsValueAssignmentCount.Store(assignmentCount)
}

func (ss *ServiceState) collectCurrentScore(ctx context.Context) {
	// collect current score
	if ss.SnapshotCurrent != nil {
		currentCost := ss.SnapshotCurrent.GetCost(ctx)
		ss.MetricsValues.MetricsValueCurrentSoftCost.Store(int64(currentCost.SoftScore))
		ss.MetricsValues.MetricsValueCurrentHardCost.Store(int64(currentCost.HardScore))
	} else {
		ss.MetricsValues.MetricsValueCurrentSoftCost.Store(0)
		ss.MetricsValues.MetricsValueCurrentHardCost.Store(0)
	}
	if ss.SnapshotFuture != nil {
		futureCost := ss.SnapshotFuture.GetCost(ctx)
		ss.MetricsValues.MetricsValueFutureSoftCost.Store(int64(futureCost.SoftScore))
		ss.MetricsValues.MetricsValueFutureHardCost.Store(int64(futureCost.HardScore))
	} else {
		ss.MetricsValues.MetricsValueFutureSoftCost.Store(0)
		ss.MetricsValues.MetricsValueFutureHardCost.Store(0)
	}
}
