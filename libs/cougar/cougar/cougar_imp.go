package cougar

import (
	"context"
	"fmt"

	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/cougar/etcdprov"
	"github.com/xinkaiwang/shardmanager/libs/unicorn/data"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"github.com/xinkaiwang/shardmanager/libs/xklib/krunloop"
)

/********************** CougarImpl **********************/

// CougarImpl implements Cougar and CriticalResource interfaces
type CougarImpl struct {
	etcdEndpoint string
	etcdSession  etcdprov.EtcdSession
	// notifyChange NotifyChangeFunc
	workerInfo  *WorkerInfo
	cougarState *CougarState
	runLoop     *krunloop.RunLoop[*CougarImpl]

	exitReason string
	exitChan   chan struct{}

	cougarApp CougarApp
}

func NewCougarImpl(ctx context.Context, etcdEndpoint string, workerInfo *WorkerInfo, app CougarApp) *CougarImpl {
	etcdSession := etcdprov.GetCurrentEtcdProvider(ctx).CreateEtcdSession(ctx)
	cougarImp := &CougarImpl{
		etcdEndpoint: etcdEndpoint,
		etcdSession:  etcdSession,
		// notifyChange: notifyChange,
		workerInfo:  workerInfo,
		cougarState: NewCougarStates(),
		exitChan:    make(chan struct{}),
		cougarApp:   app,
	}
	cougarImp.runLoop = krunloop.NewRunLoop(ctx, cougarImp, "cougar")
	go cougarImp.runLoop.Run(ctx)
	// init eph node
	ephNode := cougarImp.ToEphNode("init")
	ephPath := cougarImp.ephPath()
	cougarImp.etcdSession.PutNode(ephPath, ephNode.ToJson())
	// watch pilot node
	pilotPath := cougarImp.pilotPath()
	ch := cougarImp.etcdSession.WatchByPrefix(ctx, pilotPath, 0)
	go cougarImp.watchPilotNode(ctx, ch)
	go func() {
		// hook for session close
		reason := etcdSession.WaitForSessionClose()
		cougarImp.broadcastExit("sessionClosed:" + reason)
	}()
	// start stats report thread
	intervalMs := GetStatsReportIntervalMs()
	kcommon.ScheduleRun(intervalMs, func() {
		cougarImp.StatsReport(ctx)
	})
	return cougarImp
}

// implements CriticalResource interface
func (c *CougarImpl) IsResource() {}

// implements Cougar interface
func (c *CougarImpl) VisitState(visitor CougarStateVisitor) {
	eve := NewCougarVisitEvent(func(cougarImpl *CougarImpl) {
		result := visitor(cougarImpl.cougarState)
		if result != "" {
			// update eph node
			ephNode := cougarImpl.ToEphNode(result)
			ephPath := cougarImpl.ephPath()
			cougarImpl.etcdSession.PutNode(ephPath, ephNode.ToJson())
		}
	})
	c.runLoop.PostEvent(eve)
}

func (c *CougarImpl) VisitStateAndWait(visitor CougarStateVisitor) {
	// create a channel to wait for the event to finish
	ch := make(chan struct{})
	eve := NewCougarVisitEvent(func(cougarImpl *CougarImpl) {
		result := visitor(cougarImpl.cougarState)
		if result != "" {
			// update eph node
			ephNode := cougarImpl.ToEphNode(result)
			ephPath := cougarImpl.ephPath()
			cougarImpl.etcdSession.PutNode(ephPath, ephNode.ToJson())
		}
		close(ch)
	})
	c.runLoop.PostEvent(eve)
	<-ch
}

// implements Cougar interface
func (c *CougarImpl) RequestShutdown() chan struct{} {
	var ch chan struct{}
	c.VisitStateAndWait(func(state *CougarState) string {
		close(state.ShutDownRequest)
		ch = state.ShutdownPermited
		return "requestShutdown"
	})
	return ch
}

func (ci *CougarImpl) broadcastExit(reason string) {
	ci.exitReason = reason
	// close the exit channel
	close(ci.exitChan)
}

func (ci *CougarImpl) WaitOnExit() string {
	// wait for the exit
	<-ci.exitChan
	return ci.exitReason
}

func (ci *CougarImpl) StopLease(ctx context.Context) {
	// stop the etcd session lease
	if ci.etcdSession != nil {
		ci.etcdSession.Close(ctx)
	}
}

// etcd path is "/smg/eph/{worker_id}:{session_id}"
func (c *CougarImpl) ephPath() string {
	return fmt.Sprintf("/smg/eph/%s:%s", c.workerInfo.WorkerId, c.workerInfo.SessionId)
}

// etcd path is "/smg/pilot/{worker_id}"
func (c *CougarImpl) pilotPath() string {
	return fmt.Sprintf("/smg/pilot/%s", c.workerInfo.WorkerId)
}

func (c *CougarImpl) watchPilotNode(ctx context.Context, ch <-chan etcdprov.EtcdKvItem) {
	stop := false
	for !stop {
		select {
		case <-ctx.Done():
			stop = true
			return
		case eve, ok := <-ch:
			if !ok {
				klogging.Warning(ctx).With("worker_id", c.workerInfo.WorkerId).With("session_id", c.workerInfo.SessionId).Log("WatchPilotNodeExit", "watch pilot node exit")
				stop = true
				continue
			}
			if eve.Value == "" {
				// delete
				// this should not happen, pilot node should always exist unless we asking for shutdown
				klogging.Warning(ctx).With("pilot_path", c.pilotPath()).With("worker_id", c.workerInfo.WorkerId).With("session_id", c.workerInfo.SessionId).Log("PilotNodeFeleted", "pilot node deleted")
				continue
			}
			klogging.Info(ctx).With("pilot_path", c.pilotPath()).With("worker_id", c.workerInfo.WorkerId).With("session_id", c.workerInfo.SessionId).With("pilot", eve.Value).With("rev", eve.ModRevision).Log("PilotNodeUpdated", "pilot node updated")
			newPilotNode := cougarjson.ParsePilotNodeJson(eve.Value)
			if newPilotNode == nil {
				klogging.Warning(ctx).With("pilot_path", c.pilotPath()).With("worker_id", c.workerInfo.WorkerId).With("session_id", c.workerInfo.SessionId).Log("PilotNodeInvalid", "pilot node is not valid")
				continue
			}
			c.runLoop.PostEvent(NewPilotNodeUpdateEvent(newPilotNode))
		}
	}
}

func (c *CougarImpl) ToEphNode(updateReason string) *cougarjson.WorkerEphJson {
	ephNode := cougarjson.NewWorkerEphJson(
		string(c.workerInfo.WorkerId),
		string(c.workerInfo.SessionId),
		c.workerInfo.StartTimeMs,
		c.workerInfo.Capacity,
	)
	ephNode.AddressPort = c.workerInfo.AddressPort
	ephNode.Properties = c.workerInfo.Properties
	ephNode.StatefulType = c.workerInfo.StatefulType
	ephNode.LastUpdateAtMs = kcommon.GetWallTimeMs()
	ephNode.LastUpdateReason = updateReason
	ephNode.ReqShutDown = kcommon.BoolToInt8(c.cougarState.IsShutdownRequested())
	for shardId, shard := range c.cougarState.AllShards {
		// assign := &cougarjson.AssignmentJson{
		// 	ShardId:      string(shardId),
		// 	ReplicaIdx:   int(shard.ReplicaIdx),
		// 	AssignmentId: string(shard.AssignmentId),
		// 	State:        shard.CurrentConfirmedState,
		// 	Stats:        shard.stats,
		// }
		assign := cougarjson.NewAssignmentJson(string(shardId), int(shard.ReplicaIdx), string(shard.AssignmentId), shard.CurrentConfirmedState)
		assign.Stats = shard.stats
		ephNode.Assignments = append(ephNode.Assignments, assign)
	}
	return ephNode
}

// CougarVisitEvent implements krunloop.IEvent interface
type CougarVisitEvent struct {
	visitor func(cougarImpl *CougarImpl)
}

func NewCougarVisitEvent(visitor func(cougarImpl *CougarImpl)) *CougarVisitEvent {
	return &CougarVisitEvent{
		visitor: visitor,
	}
}

// GetName implements krunloop.IEvent interface
func (c *CougarVisitEvent) GetName() string {
	return "CougarVisitEvent"
}

// Process implements krunloop.IEvent interface
func (c *CougarVisitEvent) Process(ctx context.Context, resource *CougarImpl) {
	c.visitor(resource)
}

// PilotNodeUpdateEvent implements krunloop.IEvent interface
type PilotNodeUpdateEvent struct {
	newPilotNode *cougarjson.PilotNodeJson
}

func NewPilotNodeUpdateEvent(newPilotNode *cougarjson.PilotNodeJson) *PilotNodeUpdateEvent {
	return &PilotNodeUpdateEvent{
		newPilotNode: newPilotNode,
	}
}

// GetName implements krunloop.IEvent interface
func (c *PilotNodeUpdateEvent) GetName() string {
	return "PilotNodeUpdateEvent"
}

// Process implements krunloop.IEvent interface
func (eve *PilotNodeUpdateEvent) Process(ctx context.Context, impl *CougarImpl) {
	oldShards := map[data.ShardId]*ShardInfo{}
	for shardId, shard := range impl.cougarState.AllShards {
		oldShards[shardId] = shard
	}
	// var needRemove []data.ShardId
	var needUpdate []*cougarjson.PilotAssignmentJson
	var needAdd []*cougarjson.PilotAssignmentJson
	newAssignments := eve.newPilotNode.Assignments
	// compare old and new assignments
	for _, newAssignment := range newAssignments {
		oldAssignment, ok := oldShards[data.ShardId(newAssignment.ShardId)]
		if ok {
			// Note: 1) target state changed 2) any custom properties changed 3) assignment id changed (this is rare, but still possible)
			if oldAssignment.TargetState != newAssignment.State || !compareStringMap(oldAssignment.Properties, newAssignment.CustomProperties) || oldAssignment.AssignmentId != data.AssignmentId(newAssignment.AsginmentId) {
				// update assignment
				needUpdate = append(needUpdate, newAssignment)
			}
			// save assignment, no change
			delete(oldShards, data.ShardId(newAssignment.ShardId))
			continue
		}
		// add new assignment
		needAdd = append(needAdd, newAssignment)
	}
	// remove remaining assignments
	for shardId, shardInfo := range oldShards {
		// callback
		impl.cougarApp.DropShard(ctx, shardId, shardInfo.ChDropped)
		go func(shardInfo *ShardInfo) {
			// wait for callback
			<-shardInfo.ChDropped
			impl.VisitState(func(state *CougarState) string {
				delete(state.AllShards, shardInfo.ShardId)
				return "shardDropped"
			})
		}(shardInfo)
	}

	// update state
	for _, newAssignment := range needUpdate {
		shardInfo := impl.cougarState.AllShards[data.ShardId(newAssignment.ShardId)]
		shardInfo.CurrentConfirmedState = newAssignment.State
		shardInfo.AssignmentId = data.AssignmentId(newAssignment.AsginmentId)
		shardInfo.ReplicaIdx = data.ReplicaIdx(newAssignment.ReplicaIdx)
		shardInfo.Properties = newAssignment.CustomProperties
		// callback
		shardInfo.AppShard.UpdateShard(ctx, shardInfo)
	}
	// add new assignments
	for _, newAssignment := range needAdd {
		shardInfo := NewCougarShardInfo(data.ShardId(newAssignment.ShardId), data.ReplicaIdx(newAssignment.ReplicaIdx), data.AssignmentId(newAssignment.AsginmentId))
		shardInfo.CurrentConfirmedState = newAssignment.State
		shardInfo.Properties = newAssignment.CustomProperties
		// callback
		shardInfo.AppShard = impl.cougarApp.AddShard(ctx, shardInfo, shardInfo.ChReady)
		impl.cougarState.AllShards[data.ShardId(newAssignment.ShardId)] = shardInfo
		go func(shardInfo *ShardInfo) {
			// wait for callback
			<-shardInfo.ChReady
			impl.VisitState(func(state *CougarState) string {
				shardInfo.CurrentConfirmedState = cougarjson.CAS_Ready
				return "shardAdded"
			})
		}(shardInfo)
	}
	// shutdown permit?
	if eve.newPilotNode.ShutdownPermited == 1 && !impl.cougarState.IsShutdownPermited() {
		klogging.Info(ctx).With("worker_id", impl.workerInfo.WorkerId).With("session_id", impl.workerInfo.SessionId).Log("ShutdownPermited", "shutdown permited")
		close(impl.cougarState.ShutdownPermited)
	}
}

// return true if a and b are equal
func compareStringMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
