package main

import "testing"

// The distribution logic is the only non-trivial part of the bench harness;
// this locks the nearest-rank percentile and error counting so a report can
// never silently mislabel its own numbers.
func TestSummaryPercentiles(t *testing.T) {
	runs := []benchRun{}
	for i := 1; i <= 20; i++ {
		runs = append(runs, benchRun{LatencyMS: int64(i * 10)})
	}
	s := summarize(runs, func(r benchRun) int64 { return r.LatencyMS })
	if s.N != 20 || s.Min != 10 || s.Max != 200 {
		t.Fatalf("n/min/max: %+v", s)
	}
	// nearest-rank: p50 -> rank ceil(0.50*20)=10 -> 100; p95 -> ceil(19)=19 -> 190.
	if s.P50 != 100 || s.P95 != 190 {
		t.Fatalf("p50/p95: %+v", s)
	}

	// Errors are counted but timing still recorded.
	runs[0].Error = "boom"
	s = summarize(runs, func(r benchRun) int64 { return r.LatencyMS })
	if s.Errors != 1 {
		t.Fatalf("errors: %+v", s)
	}

	// Empty input never panics or fabricates values.
	empty := summarize(nil, func(r benchRun) int64 { return r.LatencyMS })
	if empty.N != 0 || empty.Min != 0 || empty.P95 != 0 {
		t.Fatalf("empty: %+v", empty)
	}
}
