package config

type DynamicThresholdConfigProvider func() DynamicThresholdConfig

type DynamicThresholdConfig struct {
	DynamicThresholdMax int32
	DynamicThresholdMin int32
	HalfDecayTimeSec    int32
	IncreasePerMove     int32
}
