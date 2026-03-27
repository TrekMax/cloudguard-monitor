package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Snapshot holds the latest metrics from all collectors.
type Snapshot struct {
	Timestamp time.Time
	Metrics   map[string][]*Metrics // keyed by collector name
}

// Scheduler manages periodic metric collection from multiple collectors.
type Scheduler struct {
	logger     *slog.Logger
	collectors []Collector
	mu         sync.RWMutex
	latest     *Snapshot
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewScheduler creates a new collection scheduler.
func NewScheduler(logger *slog.Logger) *Scheduler {
	return &Scheduler{
		logger: logger,
		latest: &Snapshot{
			Metrics: make(map[string][]*Metrics),
		},
	}
}

// Register adds a collector to the scheduler.
func (s *Scheduler) Register(c Collector) {
	s.collectors = append(s.collectors, c)
}

// Start begins periodic collection for all registered collectors.
func (s *Scheduler) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)

	for _, c := range s.collectors {
		s.wg.Add(1)
		go s.runCollector(ctx, c)
	}
}

// Stop halts all collection goroutines and waits for them to finish.
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

// Latest returns the most recent snapshot of all metrics.
func (s *Scheduler) Latest() *Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latest
}

func (s *Scheduler) runCollector(ctx context.Context, c Collector) {
	defer s.wg.Done()

	// Collect immediately on start
	s.collect(ctx, c)

	ticker := time.NewTicker(c.Interval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.collect(ctx, c)
		}
	}
}

func (s *Scheduler) collect(ctx context.Context, c Collector) {
	metrics, err := c.Collect(ctx)
	if err != nil {
		s.logger.Error("collection failed",
			"collector", c.Name(),
			"error", err,
		)
		return
	}

	s.mu.Lock()
	s.latest.Metrics[c.Name()] = metrics
	s.latest.Timestamp = time.Now()
	s.mu.Unlock()

	for _, m := range metrics {
		s.logger.Debug("metrics collected",
			"collector", c.Name(),
			"category", m.Category,
			"values", m.Values,
		)
	}
}
