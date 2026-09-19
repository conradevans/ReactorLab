package observability

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestResolvedRangePreflightEnforcesDerivedBucketBound(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		window  Range
		wantErr bool
	}{
		{
			name: "seven days at one second is rejected",
			window: Range{
				From: now.Add(-7 * 24 * time.Hour), To: now,
				Bucket: time.Second, MaxPoints: MaxDisplayPoints,
			},
			wantErr: true,
		},
		{
			name: "exactly 720 buckets is accepted",
			window: Range{
				From: now.Add(-720 * time.Second), To: now,
				Bucket: time.Second, MaxPoints: MaxDisplayPoints,
			},
		},
		{
			name: "partial final bucket uses ceiling",
			window: Range{
				From: now.Add(-720*time.Second - time.Nanosecond), To: now,
				Bucket: time.Second, MaxPoints: MaxDisplayPoints,
			},
			wantErr: true,
		},
		{
			name: "721 buckets is rejected",
			window: Range{
				From: now.Add(-721 * time.Second), To: now,
				Bucket: time.Second, MaxPoints: MaxDisplayPoints,
			},
			wantErr: true,
		},
		{
			name: "derived count above caller maximum is rejected",
			window: Range{
				From: now.Add(-10 * time.Minute), To: now,
				Bucket: time.Minute, MaxPoints: 9,
			},
			wantErr: true,
		},
		{
			name: "max points above global bound is rejected",
			window: Range{
				From: now.Add(-time.Minute), To: now,
				Bucket: time.Second, MaxPoints: MaxDisplayPoints + 1,
			},
			wantErr: true,
		},
		{
			name: "zero bucket is rejected",
			window: Range{
				From: now.Add(-time.Minute), To: now,
				Bucket: 0, MaxPoints: MaxDisplayPoints,
			},
			wantErr: true,
		},
		{
			name: "reversed range is rejected",
			window: Range{
				From: now, To: now.Add(-time.Minute),
				Bucket: time.Second, MaxPoints: MaxDisplayPoints,
			},
			wantErr: true,
		},
		{
			name: "zero max points is rejected",
			window: Range{
				From: now.Add(-time.Minute), To: now,
				Bucket: time.Second, MaxPoints: 0,
			},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateResolvedRange(test.window)
			if test.wantErr && !errors.Is(err, ErrInvalidRange) {
				t.Fatalf("error = %v, want ErrInvalidRange", err)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestResolvedRangePreflightAcceptsFixedPresets(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	for _, name := range []string{"15m", "1h", "6h", "24h", "7d"} {
		window, err := ResolveRange(name, now)
		if err != nil {
			t.Fatalf("%s resolve: %v", name, err)
		}
		if err := validateResolvedRange(window); err != nil {
			t.Fatalf("%s preflight: %v", name, err)
		}
	}
}

func TestAllMetricQueriesRejectOversizedRangeBeforeStoreAccess(t *testing.T) {
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	window := Range{
		From: now.Add(-7 * 24 * time.Hour), To: now,
		Bucket: time.Second, MaxPoints: MaxDisplayPoints,
	}
	query := &QueryService{store: nil}
	tests := []struct {
		name  string
		query func() error
	}{
		{
			name: "host",
			query: func() error {
				_, err := query.QueryHost(context.Background(), window)
				return err
			},
		},
		{
			name: "temperature",
			query: func() error {
				_, err := query.QueryTemperature(context.Background(), window)
				return err
			},
		},
		{
			name: "application summaries",
			query: func() error {
				_, err := query.QueryApplications(context.Background(), window)
				return err
			},
		},
		{
			name: "application detail",
			query: func() error {
				_, err := query.QueryApplication(context.Background(), "alpha", window)
				return err
			},
		},
		{
			name: "services",
			query: func() error {
				_, err := query.QueryServices(context.Background(), window)
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.query(); !errors.Is(err, ErrInvalidRange) {
				t.Fatalf("error = %v, want ErrInvalidRange", err)
			}
		})
	}
}
