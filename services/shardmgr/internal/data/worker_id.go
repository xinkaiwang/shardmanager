package data

import "strings"

type WorkerId string

type SessionId string

type WorkerFullId struct {
	WorkerId  WorkerId
	SessionId SessionId
}

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
