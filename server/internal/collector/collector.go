package collector

import (
	"context"
	"time"
)

// Metrics holds a collection of metric samples from a single collection.
type Metrics struct {
	Category  string            `json:"category"`
	Timestamp time.Time         `json:"timestamp"`
	Values    map[string]float64 `json:"values"`
	Labels    map[string]string  `json:"labels,omitempty"`
}

// Collector is the interface that all metric collectors must implement.
type Collector interface {
	// Name returns a unique identifier for this collector.
	Name() string
	// Collect gathers metrics and returns them.
	Collect(ctx context.Context) ([]*Metrics, error)
	// Interval returns how often this collector should run.
	Interval() time.Duration
}
