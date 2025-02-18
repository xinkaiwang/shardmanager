package core

import "github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"

type WorkerState struct {
	WorkerId  data.WorkerId
	SessionId data.SessionId
}

func NewWorkerState(workerId data.WorkerId, sessionId data.SessionId) *WorkerState {
	return &WorkerState{
		WorkerId:  workerId,
		SessionId: sessionId,
	}
}
