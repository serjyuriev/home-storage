package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/serjyuriev/home-storage/api/internal/models"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("not found")
var ErrInsufficientStock = errors.New("insufficient stock")
var ErrDuplicate = errors.New("duplicate")

type DBRepository struct {
	db *gorm.DB
}

func NewDBRepository(db *gorm.DB) *DBRepository {
	return &DBRepository{db: db}
}

func (r *DBRepository) GetStockStatus(ctx context.Context) ([]models.StockStatus, error) {
	var results []models.StockStatus
	return results, r.db.WithContext(ctx).Find(&results).Error
}

func (r *DBRepository) GetLowStock(ctx context.Context) ([]models.Item, error) {
	var results []models.Item
	sql := `
		SELECT id, name, category_id, qty_current, qty_restock_threshold,
		       unit, notes, updated_at
		FROM get_running_low()
	`
	return results, r.db.WithContext(ctx).Raw(sql).Scan(&results).Error
}

func (r *DBRepository) GetCategories(ctx context.Context) ([]models.Category, error) {
	var results []models.Category
	return results, r.db.WithContext(ctx).Find(&results).Error
}

func (r *DBRepository) GetItems(ctx context.Context) ([]models.ItemWithCategory, error) {
	var results []models.ItemWithCategory
	sql := `
		SELECT i.id, i.name, i.category_id,
		       c.name AS category_name, c.icon_emoji,
		       i.qty_current, i.qty_restock_threshold,
		       i.unit, i.notes, i.updated_at
		FROM items i
		LEFT JOIN categories c ON c.id = i.category_id
		ORDER BY i.name
	`
	return results, r.db.WithContext(ctx).Raw(sql).Scan(&results).Error
}

func (r *DBRepository) GetItemByID(ctx context.Context, id int) (*models.ItemDetail, error) {
	var item models.ItemWithCategory
	sql := `
		SELECT i.id, i.name, i.category_id,
		       c.name AS category_name, c.icon_emoji,
		       i.qty_current, i.qty_restock_threshold,
		       i.unit, i.notes, i.updated_at
		FROM items i
		LEFT JOIN categories c ON c.id = i.category_id
		WHERE i.id = ?
	`
	result := r.db.WithContext(ctx).Raw(sql, id).Scan(&item)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrNotFound
	}

	var daysRemaining *float64
	r.db.WithContext(ctx).Raw("SELECT estimate_days_remaining(?)", id).Scan(&daysRemaining)

	return &models.ItemDetail{ItemWithCategory: item, DaysRemaining: daysRemaining}, nil
}

func (r *DBRepository) GetMonthlyAnalytics(ctx context.Context, itemID *int) ([]models.MonthlyConsumption, error) {
	var results []models.MonthlyConsumption
	query := r.db.WithContext(ctx).Model(&models.MonthlyConsumption{}).Order("month DESC, item_name")
	if itemID != nil {
		query = query.Where("item_id = ?", *itemID)
	}
	return results, query.Find(&results).Error
}

func (r *DBRepository) CreateCategory(ctx context.Context, name, iconEmoji string) (*models.Category, error) {
	c := models.Category{Name: name, IconEmoji: iconEmoji}
	if err := r.db.WithContext(ctx).Create(&c).Error; err != nil {
		if strings.Contains(err.Error(), "duplicate key value") {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	return &c, nil
}

func (r *DBRepository) ConsumeItem(ctx context.Context, id int, amount float64, reason string) error {
	var exists int64
	if err := r.db.WithContext(ctx).Model(&models.Item{}).Where("id = ?", id).Count(&exists).Error; err != nil {
		return err
	}
	if exists == 0 {
		return ErrNotFound
	}
	if err := r.db.WithContext(ctx).Exec("CALL log_usage(?, ?, ?)", id, amount, reason).Error; err != nil {
		if strings.Contains(err.Error(), "Not enough item in stock") {
			return ErrInsufficientStock
		}
		return err
	}
	return nil
}

func (r *DBRepository) RestockItem(ctx context.Context, id int, amount float64, pricePaid *float64) error {
	var exists int64
	if err := r.db.WithContext(ctx).Model(&models.Item{}).Where("id = ?", id).Count(&exists).Error; err != nil {
		return err
	}
	if exists == 0 {
		return ErrNotFound
	}
	return r.db.WithContext(ctx).Exec("CALL restock_item(?, ?, ?)", id, amount, pricePaid).Error
}
