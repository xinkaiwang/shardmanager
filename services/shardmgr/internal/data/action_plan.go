package data

type Action struct {
	Type          ActionType    `json:"type"`
	ReplicaFullId ReplicaFullId `json:"replica"`
	Worker        WorkerFullId  `json:"worker"`
	AssignmentId  AssignmentId  `json:"assignment_id"`
	SleepMs       int           `json:"sleep_ms"`
}

type ActionType string

const (
	AT_AddShard    ActionType = "add_shard"
	AT_DropShard   ActionType = "drop_shard"
	AT_AddRouting  ActionType = "add_routing"
	AT_DropRouting ActionType = "drop_routing"
)
