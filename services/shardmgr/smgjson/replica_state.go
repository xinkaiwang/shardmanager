package smgjson

type ReplicaStateJson struct {
	ReplicaIdx  int32    `json:"idx,omitempty"`
	Assignments []string `json:"assignments,omitempty"`
	LameDuck    int8     `json:"lame_duck,omitempty"` // use int to represent bool
}

func NewReplicaStateJson() *ReplicaStateJson {
	return &ReplicaStateJson{}
}
