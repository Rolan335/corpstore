package handlers

import (
	"context"

	"github.com/gin-gonic/gin"
)

type StoragePing interface {
	Ping(ctx context.Context) error
}

type HealthHandler struct {
	db StoragePing
}

func NewHealthHandler(db StoragePing) *HealthHandler {
	return &HealthHandler{
		db: db,
	}
}

func (h *HealthHandler) Ready(c *gin.Context) {
	c.Status(200)
}

func (h *HealthHandler) Healthy(c *gin.Context) {
	// if err := h.db.Ping(c.Request.Context()); err != nil {
	// 	c.Status(500)
	// 	return
	// }
	c.Status(200)
}
