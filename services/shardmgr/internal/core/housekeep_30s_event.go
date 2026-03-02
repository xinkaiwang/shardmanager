package core

import (
	"context"
	"log/slog"
	"os"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kmetrics"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
)

// Housekeep30sEvent implements krunloop.IEvent[*ServiceState] interface
type Housekeep30sEvent struct {
	createTimeMs int64 // time when the event was created
}

func NewHousekeep30sEvent() *Housekeep30sEvent {
	return &Housekeep30sEvent{
		createTimeMs: kcommon.GetWallTimeMs(),
	}
}

func (te *Housekeep30sEvent) GetCreateTimeMs() int64 {
	return te.createTimeMs
}
func (te *Housekeep30sEvent) GetName() string {
	return "Housekeep30sEvent"
}

func (te *Housekeep30sEvent) Process(ctx context.Context, ss *ServiceState) {
	ke := kcommon.TryCatchRun(ctx, func() {
		ss.annualCheck(ctx)
		ss.hourlyCheck(ctx)
		ss.checkOrphanHats(ctx)
	})
	if ke != nil {
		slog.ErrorContext(ctx, "snapshotConverge failed",
			slog.String("event", "Housekeep30sEvent"),
			slog.Any("error", ke))
	}
	kcommon.ScheduleRun(30*1000, func() { // 30s
		ss.PostEvent(NewHousekeep30sEvent())
	})
}

func (ss *ServiceState) annualCheck(ctx context.Context) {
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
			ss.CreateSnapshotFromCurrentState(ctx) // redo for debug
			slog.ErrorContext(ctx, "found diffs",
				slog.String("event", "annualCheck"),
				slog.Any("diffs", diffs),
				slog.Any("current", oldStr),
				slog.Any("newCurrent", newStr))
			os.Exit(1)
		} else {
			slog.WarnContext(ctx, "found diffs",
				slog.String("event", "annualCheck"),
				slog.Any("diffs", diffs),
				slog.Any("current", oldStr),
				slog.Any("newCurrent", newStr),
				slog.Any("inflight", len(ss.AllMoves)))
		}
	}
	diffs = compareSnapshot(ctx, ss.GetSnapshotFutureForAny(ctx), newSnapshotFuture)
	if len(diffs) > 0 {
		if !inFlightMove {
			slog.ErrorContext(ctx, "found diffs",
				slog.String("event", "annualCheck"),
				slog.Any("diffs", diffs),
				slog.Any("future", ss.GetSnapshotFutureForAny(ctx).ToJsonString()),
				slog.Any("newFuture", newSnapshotFuture.ToJsonString()))
			os.Exit(1)
		} else {
			slog.WarnContext(ctx, "found diffs",
				slog.String("event", "annualCheck"),
				slog.Any("diffs", diffs),
				slog.Any("future", ss.GetSnapshotFutureForAny(ctx).ToJsonString()),
				slog.Any("newFuture", newSnapshotFuture.ToJsonString()),
				slog.Any("inflight", len(ss.AllMoves)))
		}
	}
	ss.SnapshotCurrent = newSnapshotCurrent
	newSnapshotFuture.Freeze()
	ss.SetSnapshotFuture(ctx, newSnapshotFuture, "annualCheck")
	slog.InfoContext(ctx, "done",
		slog.String("event", "annualCheck"),
		slog.Any("current", newSnapshotCurrent.ToShortString(ctx)),
		slog.Any("future", newSnapshotFuture.ToShortString(ctx)))
}

func (ss *ServiceState) hourlyCheck(ctx context.Context) {
	now := kcommon.GetWallTimeMs()
	if ss.LastHourlyCheckMs+60*60*1000 > now {
		return
	}
	ss.LastHourlyCheckMs = now
	slog.InfoContext(ctx, "dumping current snapshot",
		slog.String("event", "hourlyCheck"),
		slog.Any("current", ss.GetSnapshotCurrentForAny().ToJsonString()))
	slog.InfoContext(ctx, "dumping future snapshot",
		slog.String("event", "hourlyCheck"),
		slog.Any("future", ss.GetSnapshotFutureForAny(ctx).ToJsonString()))
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
		var snapshot *costfunc.Snapshot
		kmetrics.InstrumentSummaryRunVoid(ctx, "SolverGroupOnSnapshot", func() {
			snapshot = ss.GetSnapshotFutureForClone(ctx)
			snapshot.GetCost(ctx) // ensure cost is calculated
			ss.SolverGroup.OnSnapshot(ctx, snapshot, reason)
		}, reason)

		slog.InfoContext(ctx, "SolverGroup.OnSnapshot",
			slog.String("event", "broadcastSnapshot"),
			slog.Any("snapshot", snapshot.ToShortString(ctx)))
	} else {
		slog.InfoContext(ctx, "SolverGroup is nil, skip OnSnapshot",
			slog.String("event", "broadcastSnapshot"),
			slog.Any("snapshot", ss.GetSnapshotFutureForAny(ctx).ToShortString(ctx)))
	}
}
