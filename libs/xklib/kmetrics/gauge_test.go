package kmetrics

import (
	"context"
	"sort"
	"testing"

	"go.opencensus.io/metric"
)

// TestAddInt64DerivedGaugeWithLabels_MultipleCallsSameNamePreserveAllEntries
// is the regression test for the silent-overwrite bug: prior to caching the
// gauge by (registry, name), calling the helper twice for the same gaugeName
// would re-register the underlying baseMetric in OpenCensus's registry,
// orphaning the entry from the first call. Only the last entry would appear
// in scrape output.
//
// This test exercises the buggy call shape (helper invoked once per label
// value) and asserts that all entries are observable.
func TestAddInt64DerivedGaugeWithLabels_MultipleCallsSameNamePreserveAllEntries(t *testing.T) {
	r := metric.NewRegistry()
	ctx := context.Background()

	for _, phase := range []string{"buffer", "relay_to_owner"} {
		p := phase
		AddInt64DerivedGaugeWithLabels(ctx, r,
			func() int64 {
				if p == "buffer" {
					return 7
				}
				return 11
			},
			"test_phase_gauge_a",
			"two-phase test gauge",
			map[string]string{"phase": p},
		)
	}

	gotByLabel := map[string]int64{}
	for _, m := range r.Read() {
		if m.Descriptor.Name != "test_phase_gauge_a" {
			continue
		}
		for _, ts := range m.TimeSeries {
			if len(ts.LabelValues) != 1 {
				t.Fatalf("expected 1 label value per series, got %d", len(ts.LabelValues))
			}
			lv := ts.LabelValues[0].Value
			if len(ts.Points) != 1 {
				t.Fatalf("expected 1 point per series, got %d", len(ts.Points))
			}
			gotByLabel[lv] = ts.Points[0].Value.(int64)
		}
	}

	want := map[string]int64{"buffer": 7, "relay_to_owner": 11}
	if len(gotByLabel) != len(want) {
		labels := []string{}
		for k := range gotByLabel {
			labels = append(labels, k)
		}
		sort.Strings(labels)
		t.Fatalf("expected %d label entries, got %d (labels=%v)", len(want), len(gotByLabel), labels)
	}
	for k, v := range want {
		if gotByLabel[k] != v {
			t.Errorf("label %q: want %d, got %d", k, v, gotByLabel[k])
		}
	}
}

// TestAddInt64DerivedGaugeWithLabels_TwoLabelKeys_AllCombosPreserved exercises
// the helper with a 2-dimensional label space (channel × rank, like
// pipeline_channel_depth_top in mochicloud). Every combo registered must
// remain visible.
func TestAddInt64DerivedGaugeWithLabels_TwoLabelKeys_AllCombosPreserved(t *testing.T) {
	r := metric.NewRegistry()
	ctx := context.Background()

	channels := []string{"audio", "transcript"}
	ranks := []string{"1", "2", "3"}
	for _, ch := range channels {
		for _, rk := range ranks {
			c, k := ch, rk
			AddInt64DerivedGaugeWithLabels(ctx, r,
				func() int64 { return int64(len(c)*10 + len(k)) },
				"test_channel_rank_gauge",
				"two-label test gauge",
				map[string]string{"channel": c, "rank": k},
			)
		}
	}

	seen := map[string]bool{}
	for _, m := range r.Read() {
		if m.Descriptor.Name != "test_channel_rank_gauge" {
			continue
		}
		for _, ts := range m.TimeSeries {
			if len(ts.LabelValues) != 2 {
				t.Fatalf("expected 2 label values per series, got %d", len(ts.LabelValues))
			}
			// Label-key order is map-iteration-defined at registration time;
			// stitch back the values in registry-declared order.
			pair := ts.LabelValues[0].Value + "|" + ts.LabelValues[1].Value
			seen[pair] = true
		}
	}

	if got, want := len(seen), len(channels)*len(ranks); got != want {
		t.Errorf("expected %d unique label combos, got %d (seen=%v)", want, got, seen)
	}
}

// TestAddInt64DerivedGaugeWithLabels_DifferentRegistriesAreIndependent
// confirms the cache key includes the registry pointer — re-registering
// the same gauge name in a fresh registry must work, not return a stale
// gauge from another registry.
func TestAddInt64DerivedGaugeWithLabels_DifferentRegistriesAreIndependent(t *testing.T) {
	ctx := context.Background()

	r1 := metric.NewRegistry()
	r2 := metric.NewRegistry()

	AddInt64DerivedGaugeWithLabels(ctx, r1, func() int64 { return 1 },
		"test_per_registry_gauge", "x", map[string]string{"k": "v"})
	AddInt64DerivedGaugeWithLabels(ctx, r2, func() int64 { return 2 },
		"test_per_registry_gauge", "x", map[string]string{"k": "v"})

	check := func(reg *metric.Registry, want int64) {
		for _, m := range reg.Read() {
			if m.Descriptor.Name != "test_per_registry_gauge" {
				continue
			}
			if len(m.TimeSeries) != 1 {
				t.Fatalf("registry has %d series, want 1", len(m.TimeSeries))
			}
			got := m.TimeSeries[0].Points[0].Value.(int64)
			if got != want {
				t.Errorf("want %d, got %d", want, got)
			}
			return
		}
		t.Fatalf("metric not found in registry")
	}
	check(r1, 1)
	check(r2, 2)
}
