package config

type DynamicThresholdConfigProvider interface {
	GetDynamicThresholdConfig() DynamicThresholdConfig
}

type DynamicThresholdConfig struct {
	DynamicThresholdMax int32
	DynamicThresholdMin int32
	HalfDecayTimeSec    int32
	IncreasePerMove     int32
}
