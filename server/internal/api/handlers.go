package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/trekmax/cloudguard-monitor/internal/store"
)

func (s *Server) handleHealth(c *gin.Context) {
	success(c, gin.H{"status": "ok"})
}

func (s *Server) handleStatus(c *gin.Context) {
	snap := s.scheduler.Latest()

	// Build status from latest snapshot
	status := make(map[string]interface{})
	for name, metrics := range snap.Metrics {
		if len(metrics) > 0 {
			// For categories with single metrics, flatten
			if len(metrics) == 1 {
				status[name] = metrics[0].Values
			} else {
				// Multiple metrics (e.g., disk per-partition)
				items := make([]interface{}, 0, len(metrics))
				for _, m := range metrics {
					item := map[string]interface{}{
						"values": m.Values,
					}
					if len(m.Labels) > 0 {
						item["labels"] = m.Labels
					}
					items = append(items, item)
				}
				status[name] = items
			}
		}
	}

	status["timestamp"] = snap.Timestamp.Unix()
	success(c, status)
}

func (s *Server) handleMetrics(c *gin.Context) {
	q := store.MetricQuery{
		Category: c.Query("category"),
		Name:     c.Query("name"),
	}

	if start := c.Query("start"); start != "" {
		v, err := strconv.ParseInt(start, 10, 64)
		if err != nil {
			errorResponse(c, http.StatusBadRequest, "invalid start parameter")
			return
		}
		q.Start = v
	}

	if end := c.Query("end"); end != "" {
		v, err := strconv.ParseInt(end, 10, 64)
		if err != nil {
			errorResponse(c, http.StatusBadRequest, "invalid end parameter")
			return
		}
		q.End = v
	}

	if limit := c.Query("limit"); limit != "" {
		v, err := strconv.Atoi(limit)
		if err != nil {
			errorResponse(c, http.StatusBadRequest, "invalid limit parameter")
			return
		}
		q.Limit = v
	}

	records, err := s.store.QueryMetrics(q)
	if err != nil {
		s.logger.Error("query metrics failed", "error", err)
		errorResponse(c, http.StatusInternalServerError, "failed to query metrics")
		return
	}

	success(c, records)
}

func (s *Server) handleSystem(c *gin.Context) {
	success(c, s.sysInfo)
}

func (s *Server) handleProcesses(c *gin.Context) {
	snap := s.scheduler.Latest()
	procs, ok := snap.Metrics["process"]
	if !ok || len(procs) == 0 {
		success(c, []interface{}{})
		return
	}

	var result []interface{}
	for _, m := range procs {
		entry := map[string]interface{}{
			"values": m.Values,
		}
		if len(m.Labels) > 0 {
			entry["labels"] = m.Labels
		}
		result = append(result, entry)
	}

	success(c, result)
}
