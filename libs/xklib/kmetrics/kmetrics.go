package kmetrics

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/resource"
)

// Kmetric means 1 metric
// For 1 Kmetric, it may produce multiple metric name, such as "shardmgr_log_size_sum" and "shardmgr_log_size_count".
// For each metrics name, it often contains multiple time-sequence (tags), such as event="EtcdLockNotFound", event="CreatingLease", etc.
type Kmetric struct {
	mu          sync.Mutex // lock this only when adding new TimeSequence
	metricName  string
	description string
	tagNames    []string
	collection  unsafe.Pointer
	startTime   time.Time
	countOnly   bool
}

func CreateKmetric(ctx context.Context, name string, description string, tags []string) *Kmetric {
	km := &Kmetric{
		metricName:  name,
		description: description,
		tagNames:    tags,
		startTime:   time.Now(),
	}
	km.collection = unsafe.Pointer(CreateTimeSequenceCollection(km))

	// register
	GetKmetricsRegistry().RegisterKmetric(km)
	return km
}

func (km *Kmetric) CountOnly() *Kmetric {
	km.countOnly = true
	return km
}

func makeSequenceKey(tags ...string) string {
	return strings.Join(tags, "-")
}

// The tags list has to be the same len as the tagNames in Kmetric, same order as well.
func (m *Kmetric) GetTimeSequence(ctx context.Context, tags ...string) *TimeSequence {
	key := makeSequenceKey(tags...)
	collection := (*TimeSequenceCollection)(atomic.LoadPointer(&m.collection))
	sequence, ok := collection.dict[key]
	if ok {
		return sequence
	}

	// needs lock
	m.mu.Lock()
	defer m.mu.Unlock()

	// double check after lock
	collection = (*TimeSequenceCollection)(atomic.LoadPointer(&m.collection))
	sequence, ok = collection.dict[key]
	if ok {
		return sequence
	}

	// create new TimeSequence
	newCollection := CreateTimeSequenceCollection(m)
	for k, v := range collection.dict {
		newCollection.dict[k] = v
	}
	newTimeSequence := CreateTimeSequence(ctx, key, m, tags)
	newCollection.dict[key] = newTimeSequence

	// swap the new collection
	atomic.StorePointer(&m.collection, unsafe.Pointer(newCollection))
	return newTimeSequence
}

func (m *Kmetric) ReadSum() *metricdata.Metric {
	keys := make([]metricdata.LabelKey, len(m.tagNames))
	for i, tagName := range m.tagNames {
		keys[i] = metricdata.LabelKey{Key: tagName}
	}

	descriptor := metricdata.Descriptor{
		Name:        m.metricName + "_sum",
		Description: m.description,
		Unit:        metricdata.UnitDimensionless,
		Type:        metricdata.TypeCumulativeInt64,
		LabelKeys:   keys,
	}
	resource := &resource.Resource{
		Type:   "wstore",
		Labels: map[string]string{},
	}

	collection := (*TimeSequenceCollection)(m.collection)
	timeSeries := []*metricdata.TimeSeries{}
	for _, ts := range collection.dict {
		timeSeries = append(timeSeries, ts.ReadSum())
	}

	return &metricdata.Metric{
		Descriptor: descriptor,
		Resource:   resource,
		TimeSeries: timeSeries,
	}
}

func (m *Kmetric) ReadCount() *metricdata.Metric {
	keys := make([]metricdata.LabelKey, len(m.tagNames))
	for i, tagName := range m.tagNames {
		keys[i] = metricdata.LabelKey{Key: tagName}
	}

	descriptor := metricdata.Descriptor{
		Name:        m.metricName + "_count",
		Description: m.description,
		Unit:        metricdata.UnitDimensionless,
		Type:        metricdata.TypeCumulativeInt64,
		LabelKeys:   keys,
	}
	resource := &resource.Resource{
		Type:   "wstore",
		Labels: map[string]string{},
	}

	collection := (*TimeSequenceCollection)(m.collection)
	timeSeries := []*metricdata.TimeSeries{}
	for _, ts := range collection.dict {
		timeSeries = append(timeSeries, ts.ReadCount())
	}

	return &metricdata.Metric{
		Descriptor: descriptor,
		Resource:   resource,
		TimeSeries: timeSeries,
	}
}

// TimeSequenceCollection is immutable, so every time we need to add a new TimeSequence, we need to create a new one and atomic.StorePointer
type TimeSequenceCollection struct {
	parent *Kmetric
	dict   map[string]*TimeSequence // key is `-` separated tag values, order same as tagNames array
}

func CreateTimeSequenceCollection(parent *Kmetric) *TimeSequenceCollection {
	return &TimeSequenceCollection{
		parent: parent,
		dict:   map[string]*TimeSequence{},
	}
}

// 1 TimeSequence = 1 unique tag value combination
// 1 Kmetric may contain N TimeSequences
type TimeSequence struct {
	parent      *Kmetric
	key         string
	tagValues   []string
	labelValues []metricdata.LabelValue
	count       int64
	sum         int64
}

func CreateTimeSequence(ctx context.Context, key string, parent *Kmetric, tagValues []string) *TimeSequence {
	if len(tagValues) != len(parent.tagNames) {
		ne := kerror.Create("invalidTagValues", "Number of tag values does not match tag name list").
			With("expectedLen", len(parent.tagNames)).
			With("gotLen", len(tagValues))
		panic(ne)
	}
	seq := &TimeSequence{
		parent:    parent,
		key:       key,
		tagValues: tagValues,
	}

	// create/register new time sequence
	values := make([]metricdata.LabelValue, len(tagValues))
	for i, item := range tagValues {
		values[i] = metricdata.NewLabelValue(item)
	}
	seq.labelValues = values

	// 复制要在goroutine中使用的变量，避免共享可变状态
	metricName := parent.metricName
	tagKey := key
	ctxCopy := ctx

	go func() {
		// we have to defer this log operation into another goroutine to avoid dead-lock caused by re-entry
		klogging.Verbose(ctxCopy).With("gaugeName", metricName).With("tagKey", tagKey).Log("CreateTimeSequence", "")
	}()
	return seq
}

func (ts *TimeSequence) Add(val int64) {
	atomic.AddInt64(&ts.count, 1)
	atomic.AddInt64(&ts.sum, val)
}

// Touch() creates this time sequence, mostly for counter metrics. For example: you need to monitor how many times event A happens, but event A happens rarely, so you probably need a way to create this TimeSequence as 0 (before event A actually happens).
func (ts *TimeSequence) Touch() {
	// no-op
}

func (ts *TimeSequence) Get() (count int64, sum int64) {
	return atomic.LoadInt64(&ts.count), atomic.LoadInt64(&ts.sum)
}

func (ts *TimeSequence) ReadSum() *metricdata.TimeSeries {
	point := metricdata.Point{Time: time.Now(), Value: atomic.LoadInt64(&ts.sum)}
	return &metricdata.TimeSeries{
		LabelValues: ts.labelValues,
		Points:      []metricdata.Point{point},
		StartTime:   ts.parent.startTime,
	}
}

func (ts *TimeSequence) ReadCount() *metricdata.TimeSeries {
	point := metricdata.Point{Time: time.Now(), Value: atomic.LoadInt64(&ts.count)}
	return &metricdata.TimeSeries{
		LabelValues: ts.labelValues,
		Points:      []metricdata.Point{point},
		StartTime:   ts.parent.startTime,
	}
}
