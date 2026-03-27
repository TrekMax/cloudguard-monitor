package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/trekmax/cloudguard-monitor/internal/store"
)

// --- Alert Rules ---

func (s *Server) handleListAlertRules(c *gin.Context) {
	rules, err := s.store.ListAlertRules()
	if err != nil {
		s.logger.Error("list alert rules failed", "error", err)
		errorResponse(c, http.StatusInternalServerError, "failed to list rules")
		return
	}
	if rules == nil {
		rules = []store.AlertRule{}
	}
	success(c, rules)
}

func (s *Server) handleCreateAlertRule(c *gin.Context) {
	var req store.AlertRule
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Category == "" || req.Metric == "" || req.Operator == "" {
		errorResponse(c, http.StatusBadRequest, "name, category, metric, and operator are required")
		return
	}

	req.Enabled = true
	id, err := s.store.CreateAlertRule(&req)
	if err != nil {
		s.logger.Error("create alert rule failed", "error", err)
		errorResponse(c, http.StatusInternalServerError, "failed to create rule")
		return
	}

	req.ID = id
	c.JSON(http.StatusCreated, Response{
		Code:    201,
		Message: "created",
		Data:    req,
	})
}

func (s *Server) handleUpdateAlertRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid rule ID")
		return
	}

	var req store.AlertRule
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid request body")
		return
	}

	req.ID = id
	if err := s.store.UpdateAlertRule(&req); err != nil {
		s.logger.Error("update alert rule failed", "error", err)
		errorResponse(c, http.StatusInternalServerError, "failed to update rule")
		return
	}

	success(c, req)
}

func (s *Server) handleDeleteAlertRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid rule ID")
		return
	}

	if err := s.store.DeleteAlertRule(id); err != nil {
		errorResponse(c, http.StatusInternalServerError, "failed to delete rule")
		return
	}

	success(c, gin.H{"deleted": id})
}

// --- Alert Events ---

func (s *Server) handleListAlerts(c *gin.Context) {
	q := store.AlertEventQuery{
		Status: c.Query("status"),
	}

	if ruleID := c.Query("rule_id"); ruleID != "" {
		id, err := strconv.ParseInt(ruleID, 10, 64)
		if err == nil {
			q.RuleID = id
		}
	}
	if limit := c.Query("limit"); limit != "" {
		v, _ := strconv.Atoi(limit)
		q.Limit = v
	}
	if offset := c.Query("offset"); offset != "" {
		v, _ := strconv.Atoi(offset)
		q.Offset = v
	}

	events, err := s.store.ListAlertEvents(q)
	if err != nil {
		s.logger.Error("list alerts failed", "error", err)
		errorResponse(c, http.StatusInternalServerError, "failed to list alerts")
		return
	}
	if events == nil {
		events = []store.AlertEvent{}
	}
	success(c, events)
}

func (s *Server) handleAckAlert(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "invalid alert ID")
		return
	}

	if err := s.store.AckAlertEvent(id); err != nil {
		errorResponse(c, http.StatusInternalServerError, "failed to acknowledge alert")
		return
	}

	success(c, gin.H{"acknowledged": id})
}
