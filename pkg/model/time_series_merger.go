package model

import (
	"sort"
	"sync"

	typesv1 "github.com/grafana/pyroscope/api/gen/proto/go/types/v1"
)

func MergeSeries(aggregation *typesv1.TimeSeriesAggregationType, series ...[]*typesv1.Series) []*typesv1.Series {
	var m *TimeSeriesMerger
	if aggregation == nil || *aggregation == typesv1.TimeSeriesAggregationType_TIME_SERIES_AGGREGATION_TYPE_SUM {
		m = NewTimeSeriesMerger(true)
	} else {
		m = NewTimeSeriesMerger(false)
	}
	for _, s := range series {
		m.MergeTimeSeries(s)
	}
	return m.TimeSeries()
}

type TimeSeriesMerger struct {
	mu     sync.Mutex
	series map[uint64]*typesv1.Series
	sum    bool
}

// NewTimeSeriesMerger creates a new series merger. If sum is set, samples
// with matching timestamps are summed, otherwise duplicates are retained.
func NewTimeSeriesMerger(sum bool) *TimeSeriesMerger {
	return &TimeSeriesMerger{
		series: make(map[uint64]*typesv1.Series),
		sum:    sum,
	}
}

func (m *TimeSeriesMerger) MergeTimeSeries(s []*typesv1.Series) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range s {
		h := Labels(x.Labels).Hash()
		d, ok := m.series[h]
		if !ok {
			m.series[h] = x
			continue
		}
		d.Points = append(d.Points, x.Points...)
	}
}

func (m *TimeSeriesMerger) IsEmpty() bool {
	return len(m.series) == 0
}

func (m *TimeSeriesMerger) TimeSeries() []*typesv1.Series {
	if len(m.series) == 0 {
		return nil
	}
	r := make([]*typesv1.Series, len(m.series))
	var i int
	for _, s := range m.series {
		s.Points = s.Points[:m.mergePoints(s.Points)]
		r[i] = s
		i++
	}
	sort.Slice(r, func(i, j int) bool {
		return CompareLabelPairs(r[i].Labels, r[j].Labels) < 0
	})
	return r
}

func (m *TimeSeriesMerger) mergePoints(points []*typesv1.Point) int {
	l := len(points)
	if l < 2 {
		return l
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp < points[j].Timestamp
	})
	var j int
	for i := 1; i < l; i++ {
		if points[j].Timestamp != points[i].Timestamp || !m.sum {
			j++
			points[j] = points[i]
			continue
		}
		if m.sum {
			points[j].Value += points[i].Value
		}
	}
	return j + 1
}
