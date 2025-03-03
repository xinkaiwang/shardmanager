package data

import "strings"

type WorkerId string

type SessionId string

// WorkerFullId: sessionId is optional, it only exists when state is in memory
type WorkerFullId struct {
	WorkerId  WorkerId
	SessionId SessionId
}

var WorkerFullIdZero = WorkerFullId{WorkerId: "", SessionId: ""}

func NewWorkerFullId(workerId WorkerId, sessionId SessionId, stateInMemory bool) WorkerFullId {
	if stateInMemory {
		// If state is in memory, sessionId is required
		return WorkerFullId{WorkerId: workerId, SessionId: sessionId}
	}
	// If state is not in memory, sessionId is not included
	return WorkerFullId{WorkerId: workerId, SessionId: ""}
}

func (wfId WorkerFullId) String() string {
	if wfId.SessionId != "" {
		return string(wfId.WorkerId) + ":" + string(wfId.SessionId)
	}
	return string(wfId.WorkerId)
}

func WorkerFullIdParseFromString(str string) WorkerFullId {
	parts := strings.Split(str, ":")
	if len(parts) == 1 {
		return WorkerFullId{WorkerId: WorkerId(parts[0]), SessionId: ""}
	}
	return WorkerFullId{WorkerId: WorkerId(parts[0]), SessionId: SessionId(parts[1])}
}

// WorkerRuntimeId: both WorkerId and SessionId are always required
type WorkerRuntimeId struct {
	WorkerId  WorkerId
	SessionId SessionId
}

func (wrId WorkerRuntimeId) String() string {
	return string(wrId.WorkerId) + ":" + string(wrId.SessionId)
}

func WorkerRuntimeIdParseFromString(str string) WorkerRuntimeId {
	parts := strings.Split(str, ":")
	return WorkerRuntimeId{WorkerId: WorkerId(parts[0]), SessionId: SessionId(parts[1])}
}

func (wrId WorkerRuntimeId) ToWorkerFullId(stateInMemory bool) WorkerFullId {
	return NewWorkerFullId(wrId.WorkerId, wrId.SessionId, stateInMemory)
}
