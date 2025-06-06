package core

import (
	"testing"

	"github.com/xinkaiwang/shardmanager/libs/cougar/cougarjson"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

type FakeWorker struct {
	t            *testing.T
	setup        *FakeTimeTestSetup
	workerFullId data.WorkerFullId
	addrPort     string
	shutdownReq  bool
	Assignments  []*cougarjson.AssignmentJson // 模拟的分配列表
	stopped      bool                         // 是否已停止
}

func NewFakeWorker(t *testing.T, setup *FakeTimeTestSetup, workerId string, sessionId string, addrPort string) *FakeWorker {
	worker := &FakeWorker{
		t:            t,
		setup:        setup,
		workerFullId: data.NewWorkerFullId(data.WorkerId(workerId), data.SessionId(sessionId), data.ST_MEMORY),
		addrPort:     addrPort,
	}
	go worker.run()
	return worker
}

func (fw *FakeWorker) run() {
	// Create and set worker eph
	fw.setup.CreateAndSetWorkerEph(fw.t, string(fw.workerFullId.WorkerId), string(fw.workerFullId.SessionId), fw.addrPort)

	// wait for pilot node in a loop

	for !fw.stopped {
		pilotNode := fw.setup.GetPilotNode(fw.workerFullId)
		if pilotNode != nil {
			if kcommon.BoolFromInt8(pilotNode.ShutdownPermited) {
				fw.stopped = true
				fw.setup.UpdateEphNode(fw.workerFullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
					return nil
				})
				return
			}
			fw.setup.UpdateEphNode(fw.workerFullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
				// fw.Assignments = pilotNode.Assignments
				fw.Assignments = nil
				for _, assign := range pilotNode.Assignments {
					fw.Assignments = append(fw.Assignments, cougarjson.NewAssignmentJson(assign.ShardId, assign.ReplicaIdx, assign.AsginmentId, cougarjson.CAS_Ready))
				}
				wej.Assignments = fw.Assignments
				// Simulate some updates to the worker eph node
				wej.ReqShutDown = common.Int8FromBool(fw.shutdownReq)
				wej.LastUpdateAtMs = kcommon.GetWallTimeMs()
				wej.LastUpdateReason = "FakeWorker Update on pilot"
				return wej
			})
		}
		chWait := make(chan struct{})
		kcommon.ScheduleRun(1000, func() {
			close(chWait)
		})
		<-chWait
	}
}

func (fw *FakeWorker) RequestShutdown() {
	fw.shutdownReq = true
	fw.setup.UpdateEphNode(fw.workerFullId, func(wej *cougarjson.WorkerEphJson) *cougarjson.WorkerEphJson {
		wej.ReqShutDown = common.Int8FromBool(fw.shutdownReq)
		wej.Assignments = fw.Assignments
		wej.LastUpdateAtMs = kcommon.GetWallTimeMs()
		wej.LastUpdateReason = "FakeWorker shutdown requested"
		return wej
	})
}

func CountFakeWorkers(list []*FakeWorker, fn func(*FakeWorker) bool) int {
	count := 0
	for _, worker := range list {
		if worker != nil && fn(worker) {
			count++
		}
	}
	return count
}
