package kmetrics

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/resource"
)

type Khistogram struct {
	mu          sync.Mutex // lock this only when adding new TimeSequence
	metricName  string
	description string
	tagNames    []string
	buckets     []int64
	collection  unsafe.Pointer
	startTime   time.Time
}

// example buckets: []int64{1,3,10,100,1000,10000,60000}
// example buckets: []int64{2,10,50,200,1000,5000,20000}
// example buckets: []int64{1,3,10,30,100,300,1000,3000,10000,30000}
// example buckets: []int64{1,2,3,6,10,20,30,60,100,200,300,600,1000,2000,3000,6000,10000,20000,30000}
func CreateKhistogram(ctx context.Context, name string, description string, tags []string, buckets []int64) *Khistogram {
	his := &Khistogram{
		metricName:  name,
		description: description,
		tagNames:    tags,
		buckets:     buckets,
		startTime:   time.Now(),
	}
	his.collection = unsafe.Pointer(CreateHistoSequenceCollection(his))

	// register
	GetKmetricsRegistry().RegisterHistogram(his)
	return his
}

// The tags list has to be the same len as the tagNames in Kmetric, same order as well.
func (m *Khistogram) GetHistoSequence(ctx context.Context, tags ...string) *HistoSequence {
	key := makeSequenceKey(tags...)
	collection := (*HistoSequenceCollection)(atomic.LoadPointer(&m.collection))
	sequence, ok := collection.dict[key]
	if ok {
		return sequence
	}

	// needs lock
	m.mu.Lock()
	defer m.mu.Unlock()

	// double check after lock
	collection = (*HistoSequenceCollection)(atomic.LoadPointer(&m.collection))
	sequence, ok = collection.dict[key]
	if ok {
		return sequence
	}

	// create new HistoSequence
	newCollection := CreateHistoSequenceCollection(m)
	for k, v := range collection.dict {
		newCollection.dict[k] = v
	}
	newHistoSeqence := CreateHistoSequence(ctx, key, m, tags)
	newCollection.dict[key] = newHistoSeqence

	// swap the new collection
	atomic.StorePointer(&m.collection, unsafe.Pointer(newCollection))
	return newHistoSeqence
}

/*********************** HistoSequence ************************/

// HistoSequenceCollection is immutable, so every time we need to add new HistoSequence, we need to create a new one and atomic.StorePointer
type HistoSequenceCollection struct {
	parent *Khistogram
	dict   map[string]*HistoSequence // key is `-` separated tag values, order same as tagNames array
}

func CreateHistoSequenceCollection(parent *Khistogram) *HistoSequenceCollection {
	return &HistoSequenceCollection{
		parent: parent,
		dict:   map[string]*HistoSequence{},
	}
}

/*********************** HistoSequence ************************/

type HistoSequence struct {
	parent     *Khistogram
	tagValues  []string
	buckets    []*HistoBucket
	count      int64
	sum        int64
	lableValue []metricdata.LabelValue // mostly for sum/count only
}

func CreateHistoSequence(ctx context.Context, key string, parent *Khistogram, tagValues []string) *HistoSequence {
	if len(tagValues) != len(parent.tagNames) {
		klogging.Fatal(ctx).With("tagsCount", len(parent.tagNames)).With("valueCount", len(tagValues)).Log("TagCountDoesNotMatch", "")
	}
	hs := &HistoSequence{
		parent:    parent,
		tagValues: tagValues,
		buckets:   make([]*HistoBucket, 0), // last position is for "+Inf"
	}
	for _, bucket := range parent.buckets {
		hs.buckets = append(hs.buckets, CreateHistoBucket(ctx, hs, strconv.Itoa(int(bucket))))
	}
	// +Inf
	hs.buckets = append(hs.buckets, CreateHistoBucket(ctx, hs, "+Inf"))

	// create labelValue
	values := make([]metricdata.LabelValue, 0)
	for _, item := range tagValues {
		values = append(values, metricdata.NewLabelValue(item))
	}
	hs.lableValue = values

	return hs
}

func (ts *HistoSequence) Add(val int64) {
	for i, bucket := range ts.parent.buckets {
		if val < bucket {
			ts.buckets[i].counter++
		}
	}
	ts.buckets[len(ts.parent.buckets)].counter++ // "+Inf"
	ts.count++
	ts.sum += val
}

/*********************** HistoBucket ************************/
type HistoBucket struct {
	counter    int64
	bucketName string
	lableValue []metricdata.LabelValue
}

func CreateHistoBucket(ctx context.Context, parent *HistoSequence, bucketName string) *HistoBucket {
	// create/register new time sequence
	values := make([]metricdata.LabelValue, 0)
	for _, item := range parent.tagValues {
		values = append(values, metricdata.NewLabelValue(item))
	}
	// for histogram, we need an extra tag for le="2.0"
	values = append(values, metricdata.NewLabelValue(bucketName))
	return &HistoBucket{
		counter:    0,
		bucketName: bucketName,
		lableValue: values,
	}
}

func (ts *Khistogram) Read() []*metricdata.Metric {
	var list []*metricdata.Metric
	list = append(list, ts.ReadBucket())
	list = append(list, ts.ReadSum())
	list = append(list, ts.ReadCount())
	return list
}

func (m *Khistogram) ReadBucket() *metricdata.Metric {
	keys := make([]metricdata.LabelKey, len(m.tagNames)+1)
	for i, tagName := range m.tagNames {
		keys[i] = metricdata.LabelKey{Key: tagName}
	}
	keys[len(m.tagNames)] = metricdata.LabelKey{Key: "le"}

	descriptor := metricdata.Descriptor{
		Name:        m.metricName + "_bucket",
		Description: m.description,
		Unit:        metricdata.UnitDimensionless,
		Type:        metricdata.TypeCumulativeInt64,
		LabelKeys:   keys,
	}
	resource := &resource.Resource{
		Type:   "wstore",
		Labels: map[string]string{},
	}

	collection := (*HistoSequenceCollection)(m.collection)
	timeSeries := []*metricdata.TimeSeries{}
	for _, ts := range collection.dict {
		timeSeries = append(timeSeries, ts.ReadData()...)
	}

	return &metricdata.Metric{
		Descriptor: descriptor,
		Resource:   resource,
		TimeSeries: timeSeries,
	}
}

func (m *Khistogram) ReadSum() *metricdata.Metric {
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

	collection := (*HistoSequenceCollection)(m.collection)
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

func (m *Khistogram) ReadCount() *metricdata.Metric {
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

	collection := (*HistoSequenceCollection)(m.collection)
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

func (ts *HistoSequence) ReadData() []*metricdata.TimeSeries {
	now := time.Now()
	var list []*metricdata.TimeSeries
	for i, bucket := range ts.buckets {
		point := metricdata.Point{Time: now, Value: atomic.LoadInt64(&ts.buckets[i].counter)}
		item := &metricdata.TimeSeries{
			LabelValues: bucket.lableValue,
			Points:      []metricdata.Point{point},
			StartTime:   ts.parent.startTime,
		}
		list = append(list, item)
	}
	return list
}

func (ts *HistoSequence) ReadSum() *metricdata.TimeSeries {
	now := time.Now()
	point := metricdata.Point{Time: now, Value: atomic.LoadInt64(&ts.sum)}
	item := &metricdata.TimeSeries{
		LabelValues: ts.lableValue,
		Points:      []metricdata.Point{point},
		StartTime:   ts.parent.startTime,
	}
	return item
}

func (ts *HistoSequence) ReadCount() *metricdata.TimeSeries {
	now := time.Now()
	point := metricdata.Point{Time: now, Value: atomic.LoadInt64(&ts.count)}
	item := &metricdata.TimeSeries{
		LabelValues: ts.lableValue,
		Points:      []metricdata.Point{point},
		StartTime:   ts.parent.startTime,
	}
	return item
}
