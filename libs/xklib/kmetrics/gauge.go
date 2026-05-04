package kmetrics

import (
	"context"
	"sync"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"go.opencensus.io/metric"
	"go.opencensus.io/metric/metricdata"
)

// gaugeKey identifies a derived gauge uniquely by its (registry, name).
// The cache below is keyed on this so that AddInt64DerivedGaugeWithLabels
// can be called multiple times with the same gaugeName (e.g. once per
// label-value combination) without re-registering the underlying gauge.
//
// Why the cache exists: OpenCensus's *Registry.AddInt64DerivedGauge
// silently overwrites the existing baseMetric when called twice with the
// same name (see go.opencensus.io/metric/registry.go::initBaseMetric —
// the duplicate-name branch falls through to r.baseMetrics.Store(name, bm)
// instead of returning the existing baseMetric). That orphans every entry
// previously registered against the prior baseMetric, so all but the
// last UpsertEntry call disappear from scrape output. Caching the
// first-created gauge ensures every subsequent UpsertEntry call lands on
// the live baseMetric.
type gaugeKey struct {
	r    *metric.Registry
	name string
}

var (
	int64DerivedGaugeCacheMu sync.Mutex
	int64DerivedGaugeCache   = map[gaugeKey]*metric.Int64DerivedGauge{}
)

// AddInt64DerivedGaugeWithLabels registers a derived int64 gauge entry.
// Safe to call repeatedly with the same gaugeName — the second and
// subsequent calls reuse the gauge created by the first call and only
// append a new label-value entry. (See gaugeKey doc for the underlying
// OpenCensus behavior this works around.)
func AddInt64DerivedGaugeWithLabels(ctx context.Context, r *metric.Registry, fn func() int64, gaugeName string, description string, labels map[string]string) {
	labelKeys := []string{}
	labelValues := []string{}
	for k, v := range labels {
		labelKeys = append(labelKeys, k)
		labelValues = append(labelValues, v)
	}

	gauge := getOrCreateInt64DerivedGauge(r, gaugeName, description, labelKeys)
	upsertGauge(ctx, gauge, fn, gaugeName, labelValues...)
}

// getOrCreateInt64DerivedGauge returns the cached gauge for (r, name) if
// one exists, otherwise creates and caches one. The full create-and-cache
// path is mutexed so concurrent first-time registrations don't both call
// r.AddInt64DerivedGauge (which would re-trigger the silent-overwrite
// behavior we're working around).
func getOrCreateInt64DerivedGauge(r *metric.Registry, name, description string, labelKeys []string) *metric.Int64DerivedGauge {
	key := gaugeKey{r: r, name: name}

	int64DerivedGaugeCacheMu.Lock()
	defer int64DerivedGaugeCacheMu.Unlock()

	if existing, ok := int64DerivedGaugeCache[key]; ok {
		return existing
	}
	gauge, err := r.AddInt64DerivedGauge(name,
		metric.WithDescription(description),
		metric.WithUnit(metricdata.UnitDimensionless),
		metric.WithLabelKeys(labelKeys...),
	)
	if err != nil {
		panic(kerror.Create("metricProducerFail", "error creating gauge").With("gaugeName", name))
	}
	int64DerivedGaugeCache[key] = gauge
	return gauge
}

func upsertGauge(ctx context.Context, g *metric.Int64DerivedGauge, fn func() int64, gaugeName string, values ...string) {
	metricDataLabelValues := []metricdata.LabelValue{}
	for _, metricValue := range values {
		metricDataLabelValues = append(metricDataLabelValues, metricdata.NewLabelValue(metricValue))
	}

	err := g.UpsertEntry(fn, metricDataLabelValues...)
	if err != nil {
		panic(kerror.Create("UpsertEntryFail", "error gauge UpsertEntry").With("gaugeName", gaugeName))
	}
}
