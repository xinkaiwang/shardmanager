package data

type ProposalId string

type Proposal struct {
	ProposalId ProposalId
	Moves      []*Move
}

type Move struct {
	ReplicaFullId ReplicaFullId
	From          WorkerFullId
	To            WorkerFullId
}
