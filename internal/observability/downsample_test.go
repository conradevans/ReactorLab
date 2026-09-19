package observability

import (
	"context"
	"testing"
	"time"
)

func TestHostDownsamplingStaysWithinPointBound(t *testing.T) {
	store := openTestStore(t)
	start := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	samples := make([]HostSample, 0, 1000)
	for index := 0; index < 1000; index++ {
		value := float64(index % 100)
		samples = append(samples, HostSample{
			CollectedAt: start.Add(time.Duration(index) * time.Second),
			CPUPercent:  &value, MemoryTotalBytes: 100, DiskTotalBytes: 100,
		})
	}
	if err := store.InsertBatch(context.Background(), Batch{Hosts: samples}); err != nil {
		t.Fatal(err)
	}
	points, err := NewQueryService(store).QueryHost(context.Background(), Range{
		Name: "test", From: start, To: start.Add(1000 * time.Second),
		Bucket: time.Minute, MaxPoints: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 17 || len(points) > 20 {
		t.Fatalf("points = %d", len(points))
	}
	if points[0].CPUMaximum == nil || *points[0].CPUMaximum != 59 {
		t.Fatalf("first bucket maximum = %v", points[0].CPUMaximum)
	}
}

func TestApplicationDownsamplingPreservesUnhealthyStatus(t *testing.T) {
	store := openTestStore(t)
	start := time.Now().UTC().Truncate(time.Minute)
	samples := []ApplicationSample{
		{CollectedAt: start, AppID: "alpha", Name: "Alpha", Status: "healthy"},
		{CollectedAt: start.Add(10 * time.Second), AppID: "alpha", Name: "Alpha", Status: "unavailable"},
		{CollectedAt: start.Add(20 * time.Second), AppID: "alpha", Name: "Alpha", Status: "healthy"},
	}
	if err := store.InsertBatch(context.Background(), Batch{Applications: samples}); err != nil {
		t.Fatal(err)
	}
	points, err := NewQueryService(store).QueryApplication(context.Background(), "alpha", Range{
		Name: "test", From: start, To: start.Add(time.Minute), Bucket: time.Minute, MaxPoints: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 || points[0].Status != "unavailable" {
		t.Fatalf("points = %#v", points)
	}
}
