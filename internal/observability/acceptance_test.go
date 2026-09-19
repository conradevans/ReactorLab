package observability

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSafeLocalAcceptance(t *testing.T) {
	if os.Getenv("REACTORLAB_OBSERVABILITY_ACCEPTANCE") != "1" {
		t.Skip("set REACTORLAB_OBSERVABILITY_ACCEPTANCE=1 for read-only local acceptance")
	}
	path := filepath.Join(t.TempDir(), "observability.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	collector := NewCollector(store, CollectorConfig{})
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	started := time.Now().UTC()
	collector.Run(ctx)
	cancel()
	query := NewQueryService(store)
	window := Range{Name: "acceptance", From: started.Add(-time.Second), To: time.Now().UTC().Add(time.Second), Bucket: 5 * time.Second, MaxPoints: 20}
	host, err := query.QueryHost(context.Background(), window)
	if err != nil {
		t.Fatal(err)
	}
	if len(host) < 2 {
		t.Fatalf("host samples = %d", len(host))
	}
	temperature, err := query.QueryTemperature(context.Background(), window)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("temporary DB collected %d host display points and %d temperature buckets", len(host), len(temperature))
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}
