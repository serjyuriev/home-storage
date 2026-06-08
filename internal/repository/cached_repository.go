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

type CachedRepository struct {
	db  *DBRepository
	rdb *redis.Client
}

func NewCachedRepository(db *DBRepository, rdb *redis.Client) *CachedRepository {
	return &CachedRepository{db: db, rdb: rdb}
}

func (r *CachedRepository) GetStockStatus() ([]models.StockStatus, error) {
	ctx := context.Background()

	if raw, err := r.rdb.Get(ctx, keyStatus).Bytes(); err == nil {
		var cached []models.StockStatus
		if err = json.Unmarshal(raw, &cached); err == nil {
			return cached, nil
		}
		log.Printf("cache: failed to unmarshal %s: %v", keyStatus, err)
	}

	results, err := r.db.GetStockStatus()
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

func (r *CachedRepository) GetCategories() ([]models.Category, error) {
	ctx := context.Background()

	if raw, err := r.rdb.Get(ctx, keyCategories).Bytes(); err == nil {
		var cached []models.Category
		if err = json.Unmarshal(raw, &cached); err == nil {
			return cached, nil
		}
		log.Printf("cache: failed to unmarshal %s: %v", keyCategories, err)
	}

	results, err := r.db.GetCategories()
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

func (r *CachedRepository) CreateCategory(name, iconEmoji string) (*models.Category, error) {
	cat, err := r.db.CreateCategory(name, iconEmoji)
	if err != nil {
		return nil, err
	}
	if err = r.rdb.Del(context.Background(), keyCategories).Err(); err != nil {
		log.Printf("cache: failed to delete %s: %v", keyCategories, err)
	}
	return cat, nil
}

func (r *CachedRepository) ConsumeItem(id int, amount float64, reason string) error {
	if err := r.db.ConsumeItem(id, amount, reason); err != nil {
		return err
	}
	if err := r.rdb.Del(context.Background(), keyStatus).Err(); err != nil {
		log.Printf("cache: failed to delete %s: %v", keyStatus, err)
	}
	return nil
}

func (r *CachedRepository) RestockItem(id int, amount float64, pricePaid *float64) error {
	if err := r.db.RestockItem(id, amount, pricePaid); err != nil {
		return err
	}
	if err := r.rdb.Del(context.Background(), keyStatus).Err(); err != nil {
		log.Printf("cache: failed to delete %s: %v", keyStatus, err)
	}
	return nil
}

func (r *CachedRepository) GetLowStock() ([]models.Item, error) {
	return r.db.GetLowStock()
}

func (r *CachedRepository) GetItems() ([]models.ItemWithCategory, error) {
	return r.db.GetItems()
}

func (r *CachedRepository) GetItemByID(id int) (*models.ItemDetail, error) {
	return r.db.GetItemByID(id)
}

func (r *CachedRepository) GetMonthlyAnalytics(itemID *int) ([]models.MonthlyConsumption, error) {
	return r.db.GetMonthlyAnalytics(itemID)
}
