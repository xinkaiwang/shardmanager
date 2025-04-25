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
	notifyChange NotifyChangeFunc
	workerInfo   *WorkerInfo
	reqShutDown  bool
	cougarState  *CougarState
	runLoop      *krunloop.RunLoop[*CougarImpl]
}

func NewCougarImpl(ctx context.Context, etcdEndpoint string, notifyChange NotifyChangeFunc, workerInfo *WorkerInfo) *CougarImpl {
	etcdSession := etcdprov.GetCurrentEtcdProvider(ctx).CreateEtcdSession(ctx)
	cougarImp := &CougarImpl{
		etcdEndpoint: etcdEndpoint,
		etcdSession:  etcdSession,
		notifyChange: notifyChange,
		workerInfo:   workerInfo,
		cougarState:  NewCougarStates(),
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

// implements Cougar interface
func (c *CougarImpl) RequestShutdown() {
	eve := NewCougarVisitEvent(func(cougarImpl *CougarImpl) {
		cougarImpl.reqShutDown = true
		// update eph node
		ephNode := cougarImpl.ToEphNode("requestShutdown")
		ephPath := cougarImpl.ephPath()
		cougarImpl.etcdSession.PutNode(ephPath, ephNode.ToJson())
	})
	c.runLoop.PostEvent(eve)
}

// etcd path is "/smg/eph/{worker_id}:{session_id}"
func (c *CougarImpl) ephPath() string {
	return fmt.Sprintf("/smg/eph/%s", c.workerInfo.WorkerId)
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
	ephNode.ReqShutDown = kcommon.BoolToInt8(c.reqShutDown)
	for shardId, shard := range c.cougarState.AllShards {
		ephNode.Assignments = append(ephNode.Assignments, &cougarjson.AssignmentJson{
			ShardId:      string(shardId),
			ReplicaIdx:   int(shard.ReplicaIdx),
			AssignmentId: string(shard.AssignmentId),
			State:        shard.CurrentConfirmedState,
		})
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
func (c *PilotNodeUpdateEvent) Process(ctx context.Context, impl *CougarImpl) {
	oldShards := map[data.ShardId]*CougarShard{}
	for shardId, shard := range impl.cougarState.AllShards {
		oldShards[shardId] = shard
	}
	var needRemove []data.ShardId
	var needUpdate []*cougarjson.PilotAssignmentJson
	var needAdd []*cougarjson.PilotAssignmentJson
	newAssignments := c.newPilotNode.Assignments
	// compare old and new assignments
	for _, newAssignment := range newAssignments {
		oldAssignment, ok := oldShards[data.ShardId(newAssignment.ShardId)]
		if ok {
			if oldAssignment.TargetState != newAssignment.State || !compareStringMap(oldAssignment.Properties, newAssignment.CustomProperties) {
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
	for shardId := range oldShards {
		needRemove = append(needRemove, shardId)
	}

	// update state
	for _, newAssignment := range needUpdate {
		oldAssignment := impl.cougarState.AllShards[data.ShardId(newAssignment.ShardId)]
		oldAssignment.CurrentConfirmedState = newAssignment.State
		oldAssignment.Properties = newAssignment.CustomProperties
	}
	// remove old assignments
	for _, shardId := range needRemove {
		delete(impl.cougarState.AllShards, shardId)
	}
	// add new assignments
	for _, newAssignment := range needAdd {
		newShard := NewCougarShard(data.ShardId(newAssignment.ShardId), data.ReplicaIdx(newAssignment.ReplicaIdx), data.AssignmentId(newAssignment.AsginmentId))
		newShard.CurrentConfirmedState = newAssignment.State
		newShard.Properties = newAssignment.CustomProperties
		impl.cougarState.AllShards[data.ShardId(newAssignment.ShardId)] = newShard
	}
	// notify change
	if impl.notifyChange != nil {
		for _, newAssignment := range needAdd {
			impl.notifyChange(data.ShardId(newAssignment.ShardId), CA_AddShard)
		}
		for _, shardId := range needRemove {
			impl.notifyChange(shardId, CA_RemoveShard)
		}
		for _, newAssignment := range needUpdate {
			impl.notifyChange(data.ShardId(newAssignment.ShardId), CA_UpdateShard)
		}
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
