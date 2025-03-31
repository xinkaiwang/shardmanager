package smgjson

type ReplicaStateJson struct {
	ReplicaIdx  *int32   `json:"idx,omitempty"`
	Assignments []string `json:"assignments,omitempty"`
}

func NewReplicaStateJson() *ReplicaStateJson {
	return &ReplicaStateJson{}
}
