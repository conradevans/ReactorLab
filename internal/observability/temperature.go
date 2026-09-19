package observability

import "time"

type TemperatureAccumulator struct {
	start time.Time
	end   time.Time
	peak  time.Time
	min   float64
	max   float64
	sum   float64
	count int
}

func (a *TemperatureAccumulator) Add(at time.Time, celsius float64) {
	at = at.UTC()
	if a.count == 0 {
		a.start, a.end, a.peak = at, at, at
		a.min, a.max = celsius, celsius
	} else {
		a.end = at
		if celsius < a.min {
			a.min = celsius
		}
		if celsius > a.max {
			a.max, a.peak = celsius, at
		}
	}
	a.sum += celsius
	a.count++
}

func (a *TemperatureAccumulator) Flush(end time.Time) (TemperatureBucket, bool) {
	if a.count == 0 {
		return TemperatureBucket{}, false
	}
	if end.IsZero() || end.Before(a.end) {
		end = a.end
	}
	bucket := TemperatureBucket{
		BucketStart: a.start,
		BucketEnd:   end.UTC(),
		SampleCount: a.count,
		MinCelsius:  a.min,
		AvgCelsius:  a.sum / float64(a.count),
		MaxCelsius:  a.max,
		PeakAt:      a.peak,
	}
	*a = TemperatureAccumulator{}
	return bucket, true
}
