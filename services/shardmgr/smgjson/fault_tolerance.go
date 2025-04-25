package smgjson

import (
	"encoding/json"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

type FaultToleranceConfigJson struct {
	// GracePeriodSecBeforeDrain 是在 worker 掉线后，允许 worker 继续处理请求的时间
	// 在此期间，worker 仍然可以处理请求，超过此时间，我们将会开始将 worker 上的 shard 迁移到其他健康 worker 上
	GracePeriodSecBeforeDrain *int32 `json:"grace_period_sec_before_drain"` // default 0s

	// GracePeriodSecBeforeDirtyPurge 是在 worker 无法 移除 shard 的情况下，允许 worker 继续存在，不被强制删除的时间
	// 这个情况其实非常罕见，因为当一个 worker offline之后，上面的 shard 很快就会被转移到别的健康的 worker 上。所以这种情况通常只会发生在产品需要全面下线，所有的worker 都已经下线之后。
	GracePeriodSecBeforeDirtyPurge *int32 `json:"grace_period_sec_before_dirty_purge"` // default 24h
}

func (f *FaultToleranceConfigJson) ToJson() string {
	data, err := json.Marshal(f)
	if err != nil {
		ke := kerror.Wrap(err, "MarshalError", "failed to marshal FaultToleranceConfigJson", false)
		panic(ke)
	}
	return string(data)
}
