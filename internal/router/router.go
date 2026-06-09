package router

import (
	"github.com/gin-gonic/gin"
	"github.com/serjyuriev/home-storage/api/internal/handler"
)

func Setup(h *handler.Handler) *gin.Engine {
	r := gin.Default()

	v1 := r.Group("/api/v1")
	{
		v1.GET("/status", h.GetStatus)
		v1.GET("/status/low", h.GetLowStock)
		v1.GET("/categories", h.GetCategories)
		v1.POST("/categories", h.CreateCategory)
		v1.GET("/items", h.GetItems)
		v1.GET("/items/:id", h.GetItemByID)
		v1.POST("/items/:id/consume", h.ConsumeItem)
		v1.POST("/items/:id/restock", h.RestockItem)
		v1.GET("/analytics/monthly", h.GetMonthlyAnalytics)
	}

	return r
}
