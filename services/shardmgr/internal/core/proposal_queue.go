package core

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/common"
	"github.com/xinkaiwang/shardmanager/services/shardmgr/internal/costfunc"
)

type ProposalQueue struct {
	parent       *ServiceState
	maxQueueSize int
	list         []*costfunc.Proposal
}

func NewProposalQueue(ss *ServiceState, maxQueueSize int) *ProposalQueue {
	return &ProposalQueue{
		parent:       ss,
		maxQueueSize: maxQueueSize,
		list:         nil,
	}
}

func (pq *ProposalQueue) Push(ctx context.Context, p *costfunc.Proposal) common.EnqueueResult {
	// drop if proposal gain is not enough
	if !p.GetEfficiency().IsGreaterThan(costfunc.NewGain(0, 0)) {
		return common.ER_LowGain
	}
	// insert
	isLast := pq.insertProposalWithOrder(p)
	if len(pq.list) <= pq.maxQueueSize {
		return common.ER_Enqueued
	}
	if isLast {
		pq.list = pq.list[:len(pq.list)-1]
		return common.ER_LowGain
	}
	// new proposal enqueue succ, but need to drop the last proposal
	result := common.ER_Enqueued
	lastItem := pq.list[len(pq.list)-1]
	lastItem.OnClose(ctx, common.ER_LowGain)
	pq.list = pq.list[:len(pq.list)-1]
	return result
}

// return true when the new proposal is added on the last position
func (pq *ProposalQueue) insertProposalWithOrder(p *costfunc.Proposal) bool {
	// insert proposal to the queue, ordering based on the gain, highest gain first
	for i := 0; i < len(pq.list); i++ {
		if p.GetEfficiency().IsGreaterThan(pq.list[i].GetEfficiency()) {
			pq.list = append(pq.list[:i], append([]*costfunc.Proposal{p}, pq.list[i:]...)...)
			return false
		}
	}
	pq.list = append(pq.list, p)
	return true
}

func (pq *ProposalQueue) IsEmpty() bool {
	return len(pq.list) == 0
}

func (pq *ProposalQueue) Size() int {
	return len(pq.list)
}

func (pq *ProposalQueue) Pop() *costfunc.Proposal {
	if len(pq.list) == 0 {
		ke := kerror.Create("ProposalQueue", "Pop empty queue")
		panic(ke)
	}
	p := pq.list[0]
	pq.list = pq.list[1:]
	return p
}

func (pq *ProposalQueue) Peak() *costfunc.Proposal {
	if len(pq.list) == 0 {
		ke := kerror.Create("ProposalQueue", "Peak empty queue")
		panic(ke)
	}
	return pq.list[0]
}
