package repository

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/home-storage/api/internal/models"
	"github.com/redis/go-redis/v9"
)

const (
	keyStatus     = "home-storage:status"
	keyCategories = "home-storage:categories"

	ttlStatus     = 30 * time.Second
	ttlCategories = 24 * time.Hour
)

type Store interface {
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

type CachedRepository struct {
	db  Store
	rdb *redis.Client
}

func NewCachedRepository(db Store, rdb *redis.Client) *CachedRepository {
	return &CachedRepository{db: db, rdb: rdb}
}

func (r *CachedRepository) GetStockStatus(ctx context.Context) ([]models.StockStatus, error) {
	if raw, err := r.rdb.Get(ctx, keyStatus).Bytes(); err == nil {
		var cached []models.StockStatus
		if err = json.Unmarshal(raw, &cached); err == nil {
			return cached, nil
		}
		log.Printf("cache: failed to unmarshal %s: %v", keyStatus, err)
	}

	results, err := r.db.GetStockStatus(ctx)
	if err != nil {
		return nil, err
	}

	if raw, err := json.Marshal(results); err == nil {
		if err = r.rdb.Set(ctx, keyStatus, raw, ttlStatus).Err(); err != nil {
			log.Printf("cache: failed to set %s: %v", keyStatus, err)
		}
	}

	return results, nil
}

func (r *CachedRepository) GetCategories(ctx context.Context) ([]models.Category, error) {
	if raw, err := r.rdb.Get(ctx, keyCategories).Bytes(); err == nil {
		var cached []models.Category
		if err = json.Unmarshal(raw, &cached); err == nil {
			return cached, nil
		}
		log.Printf("cache: failed to unmarshal %s: %v", keyCategories, err)
	}

	results, err := r.db.GetCategories(ctx)
	if err != nil {
		return nil, err
	}

	if raw, err := json.Marshal(results); err == nil {
		if err = r.rdb.Set(ctx, keyCategories, raw, ttlCategories).Err(); err != nil {
			log.Printf("cache: failed to set %s: %v", keyCategories, err)
		}
	}

	return results, nil
}

func (r *CachedRepository) CreateCategory(ctx context.Context, name, iconEmoji string) (*models.Category, error) {
	cat, err := r.db.CreateCategory(ctx, name, iconEmoji)
	if err != nil {
		return nil, err
	}
	if err = r.rdb.Del(ctx, keyCategories).Err(); err != nil {
		log.Printf("cache: failed to delete %s: %v", keyCategories, err)
	}
	return cat, nil
}

func (r *CachedRepository) ConsumeItem(ctx context.Context, id int, amount float64, reason string) error {
	if err := r.db.ConsumeItem(ctx, id, amount, reason); err != nil {
		return err
	}
	if err := r.rdb.Del(ctx, keyStatus).Err(); err != nil {
		log.Printf("cache: failed to delete %s: %v", keyStatus, err)
	}
	return nil
}

func (r *CachedRepository) RestockItem(ctx context.Context, id int, amount float64, pricePaid *float64) error {
	if err := r.db.RestockItem(ctx, id, amount, pricePaid); err != nil {
		return err
	}
	if err := r.rdb.Del(ctx, keyStatus).Err(); err != nil {
		log.Printf("cache: failed to delete %s: %v", keyStatus, err)
	}
	return nil
}

func (r *CachedRepository) GetLowStock(ctx context.Context) ([]models.Item, error) {
	return r.db.GetLowStock(ctx)
}

func (r *CachedRepository) GetItems(ctx context.Context) ([]models.ItemWithCategory, error) {
	return r.db.GetItems(ctx)
}

func (r *CachedRepository) GetItemByID(ctx context.Context, id int) (*models.ItemDetail, error) {
	return r.db.GetItemByID(ctx, id)
}

func (r *CachedRepository) GetMonthlyAnalytics(ctx context.Context, itemID *int) ([]models.MonthlyConsumption, error) {
	return r.db.GetMonthlyAnalytics(ctx, itemID)
}
