package kmetrics

import (
	"sync"
	"time"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kcommon"
	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/resource"
)

// GaugeGroup is not designed for regular gauge style metrics (such as total in-flight count, current process mem usage, etc.) where you only need 1 tag value being reported by this process.
// Instead, GaugeGroup is designed for a group of time sequences (those time sequences may come and go over time), such as "top 20 shards" or "top 10 busy workers".

// GaugeGroup is a group of gauges that share the same gauge name but come with different tags.
type GaugeGroup struct {
	mu          sync.Mutex // lock this when read/write
	metricName  string
	description string
	tagNames    []string // 0 or more tag names
	startTime   time.Time
	dict        map[string]*GaugeTimeSequence // key is `-` separated tag values, order same as tagNames array
}

func NewGaugeGroup(name, desc string, tagNames ...string) *GaugeGroup {
	gg := &GaugeGroup{
		metricName:  name,
		description: desc,
		tagNames:    tagNames,
		startTime:   time.Now(),
		dict:        make(map[string]*GaugeTimeSequence),
	}

	// register
	GetKmetricsRegistry().RegisterGaugeGroup(gg)
	return gg
}

// GaugeGroup is designed for scenarios like "top 10 worker's current load".
// So when updating value, we never update individual values, instead, we always update all time sequences at the same time.
func (gg *GaugeGroup) UpdateValue(dict map[string]*GaugeTimeSequence) {
	kcommon.RunWithLock(&gg.mu, func() {
		gg.dict = dict
	})
}

func (gg *GaugeGroup) Read() *metricdata.Metric {
	keys := make([]metricdata.LabelKey, len(gg.tagNames))
	for i, tagName := range gg.tagNames {
		keys[i] = metricdata.LabelKey{Key: tagName}
	}

	descriptor := metricdata.Descriptor{
		Name:        gg.metricName,
		Description: gg.description,
		Unit:        metricdata.UnitDimensionless,
		Type:        metricdata.TypeGaugeInt64,
		LabelKeys:   keys,
	}
	resource := &resource.Resource{
		Type:   "wstore",
		Labels: map[string]string{},
	}

	timeSeries := []*metricdata.TimeSeries{}
	for _, ts := range gg.dict {
		timeSeries = append(timeSeries, ts.Read())
	}

	return &metricdata.Metric{
		Descriptor: descriptor,
		Resource:   resource,
		TimeSeries: timeSeries,
	}
}

// GaugeTimeSequence is one time sequence (one specific tag value).
type GaugeTimeSequence struct {
	parent      *GaugeGroup
	Key         string
	tagValues   []string
	labelValues []metricdata.LabelValue
	Value       int64
}

func NewGaugeTimeSequence(parent *GaugeGroup, value int64, tags ...string) *GaugeTimeSequence {
	if len(tags) != len(parent.tagNames) {
		ke := kerror.Create("TagsCountDoesNotMatch", "")
		panic(ke)
	}
	seq := &GaugeTimeSequence{
		parent:    parent,
		Key:       makeSequenceKey(tags...),
		tagValues: tags,
		Value:     value,
	}
	// create/register new time sequence
	values := make([]metricdata.LabelValue, len(tags))
	for i, item := range tags {
		values[i] = metricdata.NewLabelValue(item)
	}
	seq.labelValues = values
	return seq
}

func (gts *GaugeTimeSequence) Read() *metricdata.TimeSeries {
	point := metricdata.Point{Time: time.Now(), Value: gts.Value}
	return &metricdata.TimeSeries{
		LabelValues: gts.labelValues,
		Points:      []metricdata.Point{point},
		StartTime:   gts.parent.startTime,
	}
}
