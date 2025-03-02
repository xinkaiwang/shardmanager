package costfunc

import (
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/data"
)

type Proposal struct {
	Gain    Gain
	BasedOn SnapshotId

	// one of them is not nil
	SimpleMove  *SimpleMove
	SwapMove    *SwapMove
	ReplaceMove *ReplaceMove
}

type SimpleMove struct {
	Replica data.ReplicaFullId
	Src     data.WorkerFullId
	Dst     data.WorkerFullId
}

func (move *SimpleMove) GetSignature() string {
	return move.Replica.String() + "/" + move.Src.String() + "/" + move.Dst.String()
}

type SwapMove struct {
	Replica1 data.ReplicaFullId
	Replica2 data.ReplicaFullId
	Src      data.WorkerFullId
	Dst      data.WorkerFullId
}

func (move *SwapMove) GetSignature() string {
	return move.Replica1.String() + "/" + move.Replica2.String() + "/" + move.Src.String() + "/" + move.Dst.String()
}

type ReplaceMove struct {
	ReplicaOut data.ReplicaFullId
	ReplicaIn  data.ReplicaFullId
	Worker     data.WorkerFullId
}

func (move *ReplaceMove) GetSignature() string {
	return move.ReplicaOut.String() + "/" + move.ReplicaIn.String() + "/" + move.Worker.String()
}
