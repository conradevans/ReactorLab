package intelligence

import (
	"errors"
	"net/url"
	"time"

	"github.com/conradevans/ReactorLab/internal/observability"
)

const (
	MaxMetricPoints        = 240
	MaxEventResults        = 500
	MaxBackupResults       = 200
	MaxActivityResults     = 200
	DefaultEventResults    = 100
	DefaultBackupResults   = 100
	DefaultActivityResults = 100
)

var ErrInvalidWindow = errors.New("invalid MiniAI historical window")

func ResolveWindow(
	values url.Values,
	now time.Time,
) (observability.Range, Window, error) {
	now = now.UTC()
	rangeValues, hasRange := values["range"]
	fromValues, hasFrom := values["from"]
	toValues, hasTo := values["to"]

	if hasRange && (hasFrom || hasTo) {
		return observability.Range{}, Window{}, ErrInvalidWindow
	}
	if hasRange && len(rangeValues) != 1 {
		return observability.Range{}, Window{}, ErrInvalidWindow
	}
	if !hasRange && !hasFrom && !hasTo {
		rangeValues = []string{"1h"}
		hasRange = true
	}

	var resolved observability.Range
	if hasRange {
		window, err := observability.ResolveRange(rangeValues[0], now)
		if err != nil {
			return observability.Range{}, Window{}, ErrInvalidWindow
		}
		resolved = window
	} else {
		if !hasFrom || !hasTo || len(fromValues) != 1 ||
			len(toValues) != 1 {

			return observability.Range{}, Window{}, ErrInvalidWindow
		}
		from, err := time.Parse(time.RFC3339, fromValues[0])
		if err != nil {
			return observability.Range{}, Window{}, ErrInvalidWindow
		}
		to, err := time.Parse(time.RFC3339, toValues[0])
		if err != nil {
			return observability.Range{}, Window{}, ErrInvalidWindow
		}
		from, to = from.UTC(), to.UTC()
		if !from.Before(to) || to.After(now) ||
			from.Before(now.Add(-observability.MetricRetention)) ||
			to.Sub(from) > observability.MetricRetention ||
			to.Sub(from) < time.Millisecond {

			return observability.Range{}, Window{}, ErrInvalidWindow
		}
		resolved = observability.Range{From: from, To: to}
	}

	resolved.MaxPoints = MaxMetricPoints
	minimumBucket := ceilingDuration(
		resolved.To.Sub(resolved.From),
		MaxMetricPoints,
	)
	if resolved.Bucket < minimumBucket {
		resolved.Bucket = minimumBucket
	}
	if resolved.Bucket <= 0 || resolved.Bucket > resolved.To.Sub(resolved.From) {
		return observability.Range{}, Window{}, ErrInvalidWindow
	}

	metadata := Window{
		Range:         resolved.Name,
		From:          resolved.From.UTC(),
		To:            resolved.To.UTC(),
		BucketSeconds: resolved.Bucket.Seconds(),
		MaxPoints:     MaxMetricPoints,
	}
	return resolved, metadata, nil
}

func ceilingDuration(duration time.Duration, divisor int) time.Duration {
	nanoseconds := duration.Nanoseconds()
	bucket := (nanoseconds + int64(divisor) - 1) / int64(divisor)
	const millisecond = int64(time.Millisecond)
	if bucket < millisecond {
		bucket = millisecond
	}
	if remainder := bucket % millisecond; remainder != 0 {
		bucket += millisecond - remainder
	}
	return time.Duration(bucket)
}
