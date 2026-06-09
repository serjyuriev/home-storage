package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/serjyuriev/home-storage/api/internal/models"
	"github.com/serjyuriev/home-storage/api/internal/repository"
)

type CreateCategoryRequest struct {
	Name      string `json:"name"       binding:"required"`
	IconEmoji string `json:"icon_emoji"`
}

type ConsumeRequest struct {
	Amount float64 `json:"amount" binding:"gt=0"`
	Reason string  `json:"reason"`
}

type RestockRequest struct {
	Amount    float64  `json:"amount"     binding:"gt=0"`
	PricePaid *float64 `json:"price_paid"`
}

type Repository interface {
	GetStockStatus(ctx context.Context) ([]models.StockStatus, error)
	GetLowStock(ctx context.Context) ([]models.Item, error)
	GetCategories(ctx context.Context) ([]models.Category, error)
	GetItems(ctx context.Context) ([]models.ItemWithCategory, error)
	GetItemByID(ctx context.Context, id int) (*models.ItemDetail, error)
	GetMonthlyAnalytics(ctx context.Context, itemID *int) ([]models.MonthlyConsumption, error)

	CreateCategory(ctx context.Context, name, iconEmoji string) (*models.Category, error)
	ConsumeItem(ctx context.Context, id int, amount float64, reason string) error
	RestockItem(ctx context.Context, id int, amount float64, pricePaid *float64) error
}

type Handler struct {
	repo Repository
}

func New(repo Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) GetStatus(c *gin.Context) {
	results, err := h.repo.GetStockStatus(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, results)
}

func (h *Handler) GetLowStock(c *gin.Context) {
	results, err := h.repo.GetLowStock(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, results)
}

func (h *Handler) GetCategories(c *gin.Context) {
	results, err := h.repo.GetCategories(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, results)
}

func (h *Handler) GetItems(c *gin.Context) {
	results, err := h.repo.GetItems(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, results)
}

func (h *Handler) GetItemByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}

	item, err := h.repo.GetItemByID(c.Request.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handler) CreateCategory(c *gin.Context) {
	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cat, err := h.repo.CreateCategory(c.Request.Context(), req.Name, req.IconEmoji)
	if errors.Is(err, repository.ErrDuplicate) {
		c.JSON(http.StatusConflict, gin.H{"error": "category name already exists"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, cat)
}

func (h *Handler) ConsumeItem(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}
	var req ConsumeRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Reason == "" {
		req.Reason = "used"
	}
	err = h.repo.ConsumeItem(c.Request.Context(), id, req.Amount, req.Reason)
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
		return
	}
	if errors.Is(err, repository.ErrInsufficientStock) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "insufficient stock"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) RestockItem(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}
	var req RestockRequest
	if err = c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err = h.repo.RestockItem(c.Request.Context(), id, req.Amount, req.PricePaid)
	if errors.Is(err, repository.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "item not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) GetMonthlyAnalytics(c *gin.Context) {
	var itemID *int
	if s := c.Query("item_id"); s != "" {
		id, err := strconv.Atoi(s)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item_id"})
			return
		}
		itemID = &id
	}

	results, err := h.repo.GetMonthlyAnalytics(c.Request.Context(), itemID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, results)
}
