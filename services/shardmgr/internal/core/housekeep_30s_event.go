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
	// when we have in-flight moves, we don't fatal on snapshot diff. this is to avoid false positive.
	// For example, after the assignment is confirmed (in ss), but before the move is done (in snapshot current), the snapshot will be different from the current state. this is expected.
	inFlightMove := len(ss.AllMoves) > 0

	// re-create current snapshot from scratch
	newSnapshotCurrent := ss.CreateSnapshotFromCurrentState(ctx)
	// re-create future snapshot
	newSnapshotFuture := newSnapshotCurrent.Clone()
	for _, minion := range ss.AllMoves {
		minion.moveState.ApplyRemainingActions(newSnapshotFuture, costfunc.AM_Relaxed)
	}
	// compare with existing snapshot
	diffs := compareSnapshot(ctx, ss.SnapshotCurrent, newSnapshotCurrent)
	if len(diffs) > 0 {
		oldStr := ss.SnapshotCurrent.ToJsonString()
		newStr := newSnapshotCurrent.ToJsonString()
		if !inFlightMove {
			klogging.Fatal(ctx).With("diffs", diffs).With("current", oldStr).With("newCurrent", newStr).Log("annualCheckSnapshot", "found diffs")
		} else {
			klogging.Warning(ctx).With("diffs", diffs).With("current", oldStr).With("newCurrent", newStr).Log("annualCheckSnapshot", "found diffs")
		}
	}
	diffs = compareSnapshot(ctx, ss.GetSnapshotFutureForAny(), newSnapshotFuture)
	if len(diffs) > 0 {
		if !inFlightMove {
			klogging.Fatal(ctx).With("diffs", diffs).With("future", ss.GetSnapshotFutureForAny().ToJsonString()).With("newFuture", newSnapshotFuture.ToJsonString()).Log("annualCheckSnapshot", "found diffs")
		} else {
			klogging.Warning(ctx).With("diffs", diffs).With("future", ss.GetSnapshotFutureForAny().ToJsonString()).With("newFuture", newSnapshotFuture.ToJsonString()).Log("annualCheckSnapshot", "found diffs")
		}
	}
	ss.SnapshotCurrent = newSnapshotCurrent
	newSnapshotFuture.Freeze()
	ss.SetSnapshotFuture(ctx, newSnapshotFuture, "annualCheckSnapshot")
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
		ss.SolverGroup.OnSnapshot(ctx, ss.GetSnapshotFutureForClone(), reason)
		klogging.Info(ctx).With("snapshot", ss.GetSnapshotFutureForAny().ToShortString()).Log("broadcastSnapshot", "SolverGroup.OnSnapshot")
	} else {
		klogging.Info(ctx).With("snapshot", ss.GetSnapshotFutureForAny().ToShortString()).Log("broadcastSnapshot", "SolverGroup is nil, skip OnSnapshot")
	}
}
