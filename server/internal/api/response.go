package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Response is the unified API response format.
type Response struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
}

func success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:      200,
		Message:   "success",
		Data:      data,
		Timestamp: time.Now().Unix(),
	})
}

func errorResponse(c *gin.Context, code int, message string) {
	c.JSON(code, Response{
		Code:      code,
		Message:   message,
		Data:      nil,
		Timestamp: time.Now().Unix(),
	})
}
