package monitor

import (
	"errors"
	"fmt"
	"time"
)

var ErrHistoryUnavailable = errors.New("historical data unavailable")

type HistoryQuery struct {
	MetricKeys []MetricKey
	Start      time.Time
	End        time.Time
	ArchiveBit int
}

func (q HistoryQuery) Validate() error {
	if len(q.MetricKeys) < 1 || len(q.MetricKeys) > 8 {
		return errors.New("history requires between 1 and 8 metrics")
	}
	seen := map[MetricKey]struct{}{}
	for _, key := range q.MetricKeys {
		if !IsKnownMetricKey(key) {
			return fmt.Errorf("unknown metric key %q", key)
		}
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate metric key %q", key)
		}
		seen[key] = struct{}{}
	}
	if q.Start.IsZero() || q.End.IsZero() || !q.End.After(q.Start) {
		return errors.New("history start/end range is invalid")
	}
	if q.End.Sub(q.Start) > 31*24*time.Hour {
		return errors.New("history range must not exceed 31 days")
	}
	if q.ArchiveBit < 1 || q.ArchiveBit > 3 {
		return errors.New("history archiveBit must be 1 (minute), 2 (hourly), or 3 (daily)")
	}
	return nil
}

type HistoryPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Quality   Quality   `json:"quality"`
}

type HistorySeries struct {
	MetricKey MetricKey      `json:"metricKey"`
	Unit      string         `json:"unit,omitempty"`
	Points    []HistoryPoint `json:"points"`
}

type HistorySnapshot struct {
	GeneratorID string          `json:"generatorId"`
	Start       time.Time       `json:"start"`
	End         time.Time       `json:"end"`
	ArchiveBit  int             `json:"archiveBit"`
	Series      []HistorySeries `json:"series"`
}
