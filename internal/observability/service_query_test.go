package observability

import (
	"context"
	"testing"
	"time"
)

func TestServiceDownsamplingPreservesOutage(t *testing.T) {
	store := openTestStore(t)
	start := time.Now().UTC().Truncate(time.Minute)
	samples := []ServiceSample{
		{CollectedAt: start, ServiceID: "minibase", Name: "MiniBase", Available: true, Status: "healthy"},
		{CollectedAt: start.Add(10 * time.Second), ServiceID: "minibase", Name: "MiniBase", Available: false, Status: "unavailable"},
		{CollectedAt: start.Add(20 * time.Second), ServiceID: "minibase", Name: "MiniBase", Available: true, Status: "healthy"},
	}
	if err := store.InsertBatch(context.Background(), Batch{Services: samples}); err != nil {
		t.Fatal(err)
	}
	series, err := NewQueryService(store).QueryServices(context.Background(), Range{
		Name: "test", From: start, To: start.Add(time.Minute), Bucket: time.Minute, MaxPoints: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(series) != 1 || len(series[0].Points) != 1 || series[0].Points[0].Available || series[0].Points[0].Status != "unavailable" {
		t.Fatalf("series = %#v", series)
	}
}
