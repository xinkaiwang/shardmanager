package data

import (
	"strings"
)

type WorkerId string

type SessionId string

type StatefulType string

const (
	// ST_STATELESS     ServiceType = "stateless"     // 无状态，无需热身。迁移成本为0
	ST_MEMORY     StatefulType = "state_in_mem" // 弱状态，状态存在于内存。热身需要数秒,若掉电后需要重新热身。
	ST_HARD_DRIVE StatefulType = "state_in_hd"  // 强状态，状态存在于硬盘。热身需要数分钟/小时,但掉电重启后不需要重新热身
)

// WorkerFullId: sessionId is optional, it only exists when state is in memory
type WorkerFullId struct {
	WorkerId  WorkerId
	SessionId SessionId
}

var WorkerFullIdZero = WorkerFullId{WorkerId: "", SessionId: ""}

func NewWorkerFullId(workerId WorkerId, sessionId SessionId, statefulType StatefulType) WorkerFullId {
	if statefulType == ST_MEMORY {
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

func (wrId WorkerRuntimeId) ToWorkerFullId(statefulType StatefulType) WorkerFullId {
	return NewWorkerFullId(wrId.WorkerId, wrId.SessionId, statefulType)
}
