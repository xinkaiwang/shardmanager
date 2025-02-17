package core

type WorkerId string

type SessionId string

type WorkerFullId string

func NewWorkerFullId(workerId WorkerId, sessionId SessionId, isMemStateful bool) WorkerFullId {
	if isMemStateful {
		return WorkerFullId(string(workerId) + ":" + string(sessionId))
	}
	return WorkerFullId(workerId)
}
