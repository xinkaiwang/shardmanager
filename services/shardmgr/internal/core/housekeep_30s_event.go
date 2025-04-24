package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
)

// Housekeep30sEvent implements krunloop.IEvent[*ServiceState] interface
type Housekeep30sEvent struct {
}

func NewHousekeep30sEvent() *Housekeep30sEvent {
	return &Housekeep30sEvent{}
}

func (te *Housekeep30sEvent) GetName() string {
	return "Housekeep30sEvent"
}

func (te *Housekeep30sEvent) Process(ctx context.Context, ss *ServiceState) {
	ke := kcommon.TryCatchRun(ctx, func() {
		ss.annualCheckSnapshot(ctx)
	})
	if ke != nil {
		klogging.Error(ctx).With("error", ke).Log("Housekeep30sEvent", "annualCheckSnapshot failed")
	}
	kcommon.ScheduleRun(30*1000, func() { // 30s
		ss.PostEvent(NewHousekeep30sEvent())
	})
}

func (ss *ServiceState) annualCheckSnapshot(ctx context.Context) {
	// re-create current snapshot from scratch
	newSnapshotCurrent := ss.CreateSnapshotFromCurrentState(ctx)
	// re-create future snapshot
	newSnapshotFuture := newSnapshotCurrent.Clone()
	for _, minion := range ss.AllMoves {
		minion.moveState.ApplyRemainingActions(newSnapshotFuture, costfunc.AM_Relaxed)
	}
	// compare with existing snapshot
	diffs := compareSnapshot(ctx, ss.GetSnapshotCurrent(), newSnapshotCurrent)
	if len(diffs) > 0 {
		klogging.Fatal(ctx).With("diffs", diffs).With("current", ss.SnapshotCurrent.ToJsonString()).With("newCurrent", newSnapshotCurrent.ToJsonString()).Log("annualCheckSnapshot", "found diffs")
	}
	diffs = compareSnapshot(ctx, ss.SnapshotFuture, newSnapshotFuture)
	if len(diffs) > 0 {
		klogging.Fatal(ctx).With("diffs", diffs).With("future", ss.SnapshotFuture.ToJsonString()).With("newFuture", newSnapshotFuture.ToJsonString()).Log("annualCheckSnapshot", "found diffs")
	}
	ss.SetSnapshotCurrent(ctx, newSnapshotCurrent, "annualCheckSnapshot")
	ss.SnapshotFuture = newSnapshotFuture.Freeze()
	klogging.Info(ctx).With("current", newSnapshotCurrent.ToShortString()).With("future", newSnapshotFuture.ToShortString()).Log("annualCheckSnapshot", "done")
}

// return list of differences
func compareSnapshot(ctx context.Context, snap1 *costfunc.Snapshot, snap2 *costfunc.Snapshot) []string {
	var diffs []string
	diffs = append(diffs, snap1.AllWorkers.Compare(snap2.AllWorkers)...)
	diffs = append(diffs, snap1.AllShards.Compare(snap2.AllShards)...)
	diffs = append(diffs, snap1.AllAssignments.Compare(snap2.AllAssignments)...)
	return diffs
}

func (ss *ServiceState) broadcastSnapshot(ctx context.Context, reason string) {
	if ss.SolverGroup != nil {
		ss.SolverGroup.OnSnapshot(ctx, ss.SnapshotFuture, reason)
		klogging.Info(ctx).With("snapshot", ss.SnapshotFuture.ToShortString()).Log("broadcastSnapshot", "SolverGroup.OnSnapshot")
	} else {
		klogging.Info(ctx).With("snapshot", ss.SnapshotFuture.ToShortString()).Log("broadcastSnapshot", "SolverGroup is nil, skip OnSnapshot")
	}
}
