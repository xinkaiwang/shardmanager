package kmetrics

import (
	"context"

	"github.com/xinkaiwang/shardmanager/libs/xklib/kerror"
	"go.opencensus.io/metric"
	"go.opencensus.io/metric/metricdata"
)

func AddInt64DerivedGaugeWithLabels(ctx context.Context, r *metric.Registry, fn func() int64, gaugeName string, description string, labels map[string]string) {
	labelKeys := []string{}
	labelValues := []string{}
	for k, v := range labels {
		labelKeys = append(labelKeys, k)
		labelValues = append(labelValues, v)
	}

	gauge, err := r.AddInt64DerivedGauge(gaugeName,
		metric.WithDescription(description),
		metric.WithUnit(metricdata.UnitDimensionless),
		metric.WithLabelKeys(labelKeys...),
	)
	if err != nil {
		panic(kerror.Create("metricProducerFail", "error creating gauge").With("gaugeName", gaugeName))
	}
	upsertGauge(ctx, gauge, fn, gaugeName, labelValues...)
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
