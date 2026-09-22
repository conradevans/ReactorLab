package intelligence

import (
	"net/url"
	"testing"
	"time"
)

func TestResolveWindowSupportsNamedAndExplicitWindows(t *testing.T) {
	now := time.Date(2026, 9, 22, 18, 0, 0, 0, time.UTC)

	named, metadata, err := ResolveWindow(
		url.Values{"range": []string{"6h"}},
		now,
	)
	if err != nil {
		t.Fatalf("named range error: %v", err)
	}
	if named.Name != "6h" || !named.From.Equal(now.Add(-6*time.Hour)) ||
		!named.To.Equal(now) || metadata.Range != "6h" {

		t.Fatalf("named range = %#v, metadata = %#v", named, metadata)
	}

	explicit, explicitMetadata, err := ResolveWindow(url.Values{
		"from": []string{"2026-09-22T10:00:00-04:00"},
		"to":   []string{"2026-09-22T10:20:00-04:00"},
	}, now)
	if err != nil {
		t.Fatalf("explicit range error: %v", err)
	}
	wantFrom := time.Date(2026, 9, 22, 14, 0, 0, 0, time.UTC)
	wantTo := wantFrom.Add(20 * time.Minute)
	if !explicit.From.Equal(wantFrom) || explicit.From.Location() != time.UTC ||
		!explicit.To.Equal(wantTo) || explicit.To.Location() != time.UTC ||
		!explicitMetadata.From.Equal(wantFrom) ||
		!explicitMetadata.To.Equal(wantTo) {

		t.Fatalf(
			"explicit range = %#v, metadata = %#v",
			explicit,
			explicitMetadata,
		)
	}
}

func TestResolveWindowRejectsInvalidExplicitWindows(t *testing.T) {
	now := time.Date(2026, 9, 22, 18, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		values url.Values
	}{
		{
			name: "missing to",
			values: url.Values{
				"from": []string{"2026-09-22T17:00:00Z"},
			},
		},
		{
			name: "missing from",
			values: url.Values{
				"to": []string{"2026-09-22T18:00:00Z"},
			},
		},
		{
			name: "equal endpoints",
			values: url.Values{
				"from": []string{"2026-09-22T17:00:00Z"},
				"to":   []string{"2026-09-22T17:00:00Z"},
			},
		},
		{
			name: "reverse endpoints",
			values: url.Values{
				"from": []string{"2026-09-22T18:00:00Z"},
				"to":   []string{"2026-09-22T17:00:00Z"},
			},
		},
		{
			name: "over seven days",
			values: url.Values{
				"from": []string{"2026-09-15T17:59:59Z"},
				"to":   []string{"2026-09-22T18:00:00Z"},
			},
		},
		{
			name: "future only",
			values: url.Values{
				"from": []string{"2026-09-22T19:00:00Z"},
				"to":   []string{"2026-09-22T20:00:00Z"},
			},
		},
		{
			name: "named and explicit",
			values: url.Values{
				"range": []string{"1h"},
				"from":  []string{"2026-09-22T17:00:00Z"},
				"to":    []string{"2026-09-22T18:00:00Z"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := ResolveWindow(test.values, now); err == nil {
				t.Fatal("ResolveWindow() error = nil")
			}
		})
	}
}

func TestResolveWindowAlwaysBoundsMetricPointCount(t *testing.T) {
	now := time.Date(2026, 9, 22, 18, 0, 0, 0, time.UTC)
	for _, values := range []url.Values{
		{"range": []string{"1h"}},
		{"range": []string{"7d"}},
		{
			"from": []string{"2026-09-21T18:00:00Z"},
			"to":   []string{"2026-09-22T18:00:00Z"},
		},
	} {
		window, metadata, err := ResolveWindow(values, now)
		if err != nil {
			t.Fatalf("ResolveWindow(%v) error: %v", values, err)
		}
		duration := window.To.Sub(window.From)
		points := int(duration / window.Bucket)
		if duration%window.Bucket != 0 {
			points++
		}
		if points > MaxMetricPoints ||
			window.MaxPoints != MaxMetricPoints ||
			metadata.MaxPoints != MaxMetricPoints {

			t.Fatalf(
				"range %v permits %d points: %#v",
				values,
				points,
				window,
			)
		}
	}
}
