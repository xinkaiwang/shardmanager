package cougar

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/xinkaiwang/shardmanager/libs/unicorn/data"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
)

const (
	QpmMaxBlockSize = 10
	QpmIntervalSec  = 10 // seconds
)

type ShardQpm struct {
	shardId        data.ShardId
	currentCounter atomic.Int64
	mu             sync.Mutex
	blockChain     []int64
	stop           bool
}

func NewShardQpm(ctx context.Context, shardId data.ShardId) *ShardQpm { // shardId for log/metrics use only
	sq := &ShardQpm{
		shardId: shardId,
	}
	kcommon.ScheduleRun(QpmIntervalSec*1000, func() {
		sq.Checkin(ctx)
	})
	klogging.Info(ctx).With("shardId", shardId).Log("ShardQpm", "NewShardQpm")
	return sq
}

func (sq *ShardQpm) Inc(n int64) {
	sq.currentCounter.Add(n)
}

func (sq *ShardQpm) Checkin(ctx context.Context) {
	if sq.stop {
		klogging.Info(ctx).With("shardId", sq.shardId).Log("ShardQpm", "Checkin stopped")
		return
	}
	sq.checkin(ctx)
	kcommon.ScheduleRun(QpmIntervalSec*1000, func() {
		sq.Checkin(ctx)
	})
}

func (sq *ShardQpm) checkin(ctx context.Context) {
	sq.mu.Lock()
	defer sq.mu.Unlock()

	if len(sq.blockChain) >= QpmMaxBlockSize {
		sq.blockChain = sq.blockChain[1:]
	}
	current := sq.currentCounter.Swap(0)
	sq.blockChain = append(sq.blockChain, current)
	klogging.Info(ctx).With("current", current).With("blockChain", sq.blockChain).With("shardId", sq.shardId).Log("ShardQpm", "checkin")
}

// return avg qpm in last 60 seconds
func (sq *ShardQpm) GetQpm() int64 {
	sq.mu.Lock()
	defer sq.mu.Unlock()

	if len(sq.blockChain) == 0 {
		return 0
	}
	if len(sq.blockChain) > 6 {
		return Sum(sq.blockChain)
	}
	startPos := len(sq.blockChain) - 6
	if startPos < 0 {
		startPos = 0
	}
	return Sum(sq.blockChain[startPos:])
}

func Sum(list []int64) int64 {
	var sum int64
	for _, v := range list {
		sum += v
	}
	return sum
}

func (sq *ShardQpm) Stop() {
	sq.stop = true
}
