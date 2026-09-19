package observability

import (
	"errors"
	"time"
)

var ErrInvalidRange = errors.New("invalid observability range")

func ResolveRange(name string, now time.Time) (Range, error) {
	now = now.UTC()
	var duration, bucket time.Duration
	switch name {
	case "15m":
		duration, bucket = 15*time.Minute, 5*time.Second
	case "1h":
		duration, bucket = time.Hour, 5*time.Second
	case "6h":
		duration, bucket = 6*time.Hour, time.Minute
	case "24h":
		duration, bucket = 24*time.Hour, 3*time.Minute
	case "7d":
		duration, bucket = 7*24*time.Hour, 15*time.Minute
	default:
		return Range{}, ErrInvalidRange
	}
	return Range{Name: name, From: now.Add(-duration), To: now, Bucket: bucket, MaxPoints: MaxDisplayPoints}, nil
}

func ValidateWindow(from, to time.Time, maxPoints int) error {
	if from.IsZero() || to.IsZero() || !from.Before(to) || to.Sub(from) > MetricRetention || maxPoints < 1 || maxPoints > MaxDisplayPoints {
		return ErrInvalidRange
	}
	return nil
}

func rate(previous, current uint64, elapsed time.Duration) *float64 {
	if elapsed <= 0 || current < previous {
		return nil
	}
	value := float64(current-previous) / elapsed.Seconds()
	return &value
}
