package kmetrics

import (
	"context"
	"sync"
	"unsafe"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"github.com/xinkaiwang/shardmanager/libs/xklib/klogging"
	"go.opencensus.io/metric/metricdata"
)

// KmetricsRegistry implements the metricproducer.Producer interface.
type KmetricsRegistry struct {
	mu         sync.Mutex // lock this only when trying to add new TimeSequence
	collection unsafe.Pointer
	globalTags map[string]string

	// to detect potential tag name conflict for globalTags.
	allTagNames map[string]string // key is tagName, value is metric name
}

// NewKmetricsRegistry initializes a new KmetricsRegistry.
func NewKmetricsRegistry() *KmetricsRegistry {
	return &KmetricsRegistry{
		collection:  unsafe.Pointer(CreateKmetricsCollection()),
		globalTags:  make(map[string]string),
		allTagNames: make(map[string]string),
	}
}

// RegisterMetric registers a new Kmetric.
func (registry *KmetricsRegistry) RegisterKmetric(km *Kmetric) {
	registry.checkForTagNameConflicts(km.tagNames)

	registry.mu.Lock()
	defer registry.mu.Unlock()

	for _, tagName := range km.tagNames {
		registry.allTagNames[tagName] = km.metricName
	}

	oldCollection := (*KmetricsCollection)(registry.collection)
	newCollection := oldCollection.Clone()
	newCollection.dict[km.metricName] = km
	registry.collection = unsafe.Pointer(newCollection)
}

// RegisterHistogram registers a new Khistogram.
func (registry *KmetricsRegistry) RegisterHistogram(nh *Khistogram) {
	registry.checkForTagNameConflicts(nh.tagNames)

	registry.mu.Lock()
	defer registry.mu.Unlock()

	for _, tagName := range nh.tagNames {
		registry.allTagNames[tagName] = nh.metricName
	}

	oldCollection := (*KmetricsCollection)(registry.collection)
	newCollection := oldCollection.Clone()
	newCollection.histo[nh.metricName] = nh
	registry.collection = unsafe.Pointer(newCollection)
}

// RegisterGaugeGroup registers a new GaugeGroup.
func (registry *KmetricsRegistry) RegisterGaugeGroup(gg *GaugeGroup) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	oldCollection := (*KmetricsCollection)(registry.collection)
	newCollection := oldCollection.Clone()
	newCollection.gaugeGroups[gg.metricName] = gg
	registry.collection = unsafe.Pointer(newCollection)
}

// Read returns all registered metrics.
func (registry *KmetricsRegistry) Read() []*metricdata.Metric {
	collection := (*KmetricsCollection)(registry.collection)
	list := []*metricdata.Metric{}

	for _, v := range collection.dict {
		list = append(list, registry.attachGlobalTags(v.ReadCount()))
		if !v.countOnly {
			list = append(list, registry.attachGlobalTags(v.ReadSum()))
		}
	}
	for _, v := range collection.gaugeGroups {
		list = append(list, registry.attachGlobalTags(v.Read()))
	}
	for _, v := range collection.histo {
		for _, item := range v.Read() {
			list = append(list, registry.attachGlobalTags(item))
		}
	}
	return list
}

// attachGlobalTags adds global tags to a metric.
func (registry *KmetricsRegistry) attachGlobalTags(metric *metricdata.Metric) *metricdata.Metric {
	for key, value := range registry.globalTags {
		metric.Descriptor.LabelKeys = append(metric.Descriptor.LabelKeys, metricdata.LabelKey{Key: key})
		for _, ts := range metric.TimeSeries {
			ts.LabelValues = append(ts.LabelValues, metricdata.NewLabelValue(value))
		}
	}
	return metric
}

// KmetricsCollection is immutable; a new collection is created for new Kmetrics.
type KmetricsCollection struct {
	dict        map[string]*Kmetric
	histo       map[string]*Khistogram
	gaugeGroups map[string]*GaugeGroup
}

// CreateKmetricsCollection initializes a new KmetricsCollection.
func CreateKmetricsCollection() *KmetricsCollection {
	return &KmetricsCollection{
		dict:        make(map[string]*Kmetric),
		histo:       make(map[string]*Khistogram),
		gaugeGroups: make(map[string]*GaugeGroup),
	}
}

var kmetricsRegistry = NewKmetricsRegistry()

// GetKmetricsRegistry returns the singleton instance of KmetricsRegistry.
func GetKmetricsRegistry() *KmetricsRegistry {
	return kmetricsRegistry
}

// Clone creates a copy of the KmetricsCollection.
func (collection *KmetricsCollection) Clone() *KmetricsCollection {
	newCollection := CreateKmetricsCollection()
	for k, v := range collection.dict {
		newCollection.dict[k] = v
	}
	for k, v := range collection.histo {
		newCollection.histo[k] = v
	}
	for k, v := range collection.gaugeGroups {
		newCollection.gaugeGroups[k] = v
	}
	return newCollection
}

// AddGlobalTag adds a global tag for all metrics.
func (registry *KmetricsRegistry) AddGlobalTag(key, value string) {
	if metricName, exists := registry.allTagNames[key]; exists {
		klogging.Fatal(context.Background()).With("tagName", key).With("metricName", metricName).Log("tagNameConflict", "global tag name should not conflict with any other existing metrics tags")
	}
	registry.globalTags[key] = value
}

// checkForTagNameConflicts checks for conflicts with global tags.
func (registry *KmetricsRegistry) checkForTagNameConflicts(tagNames []string) {
	for _, tagName := range tagNames {
		if _, exists := registry.globalTags[tagName]; exists {
			ne := kerror.Create("tagNameConflict", "").With("tagName", tagName)
			klogging.Fatal(context.Background()).Log("tagNameConflict", "")
			panic(ne)
		}
	}
}
