package smgjson

import (
	"encoding/json"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
)

type DynamicThresholdConfigJson struct {
	// DynamicThresholdMax 是动态阈值的最大值
	DynamicThresholdMax *int32 `json:"dynamic_threshold_max"`
	// DynamicThresholdMin 是动态阈值的最小值
	DynamicThresholdMin *int32 `json:"dynamic_threshold_min"`
	// HalfDecayTimeSec 是动态阈值的半衰期
	HalfDecayTimeSec *int32 `json:"half_decay_time_sec"`
	// IncreasePerMove 是每次迁移增加的动态阈值
	IncreasePerMove *int32 `json:"increase_per_move"`
}

func (d *DynamicThresholdConfigJson) ToJson() string {
	data, err := json.Marshal(d)
	if err != nil {
		ke := kerror.Wrap(err, "MarshalError", "failed to marshal DynamicThresholdConfigJson", false)
		panic(ke)
	}
	return string(data)
}
