package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/krunloop"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
)

type SnapshotOperationManager struct {
	parent       krunloop.EventPoster[*ServiceState]
	batchManager *BatchManager
	operations   []func(snapshot *costfunc.Snapshot)
}

func NewSnapshotBatchManager(parent krunloop.EventPoster[*ServiceState]) *SnapshotOperationManager {
	som := &SnapshotOperationManager{
		parent: parent,
	}
	som.batchManager = NewBatchManager(parent, 10, "SnapshotOperationManager", som.onCallback)
	return som
}

func (som *SnapshotOperationManager) onCallback(ctx context.Context, ss *ServiceState) {
	if len(som.operations) == 0 {
		return
	}
	current := ss.GetSnapshotCurrent().Clone()
	future := ss.SnapshotFuture.Clone()
	for _, operation := range som.operations {
		operation(current)
		operation(future)
	}
	ss.SetSnapshotCurrent(ctx, current.CompactAndFreeze(), "SnapshotOperationManager")
	ss.SnapshotFuture = future.CompactAndFreeze()
	som.operations = nil
	ss.broadcastSnapshot(ctx, "SnapshotOperationManager")
}

func (som *SnapshotOperationManager) TrySchedule(ctx context.Context, operation func(snapshot *costfunc.Snapshot), reason string) {
	krunloop.VisitResource(som.parent, func(ss *ServiceState) {
		som.operations = append(som.operations, operation)
		som.batchManager.TryScheduleInternal(ctx, reason)
	})
}
