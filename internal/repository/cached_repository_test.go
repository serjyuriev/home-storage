package repository_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/serjyuriev/home-storage/api/internal/models"
	"github.com/serjyuriev/home-storage/api/internal/repository"
)

type mockStore struct {
	getStockStatusFn      func(ctx context.Context) ([]models.StockStatus, error)
	getLowStockFn         func(ctx context.Context) ([]models.Item, error)
	getCategoriesFn       func(ctx context.Context) ([]models.Category, error)
	getItemsFn            func(ctx context.Context) ([]models.ItemWithCategory, error)
	getItemByIDFn         func(ctx context.Context, id int) (*models.ItemDetail, error)
	getMonthlyAnalyticsFn func(ctx context.Context, itemID *int) ([]models.MonthlyConsumption, error)
	createCategoryFn      func(ctx context.Context, name, iconEmoji string) (*models.Category, error)
	consumeItemFn         func(ctx context.Context, id int, amount float64, reason string) error
	restockItemFn         func(ctx context.Context, id int, amount float64, pricePaid *float64) error
}

func (m *mockStore) GetStockStatus(ctx context.Context) ([]models.StockStatus, error) {
	return m.getStockStatusFn(ctx)
}
func (m *mockStore) GetLowStock(ctx context.Context) ([]models.Item, error) {
	return m.getLowStockFn(ctx)
}
func (m *mockStore) GetCategories(ctx context.Context) ([]models.Category, error) {
	return m.getCategoriesFn(ctx)
}
func (m *mockStore) GetItems(ctx context.Context) ([]models.ItemWithCategory, error) {
	return m.getItemsFn(ctx)
}
func (m *mockStore) GetItemByID(ctx context.Context, id int) (*models.ItemDetail, error) {
	return m.getItemByIDFn(ctx, id)
}
func (m *mockStore) GetMonthlyAnalytics(ctx context.Context, itemID *int) ([]models.MonthlyConsumption, error) {
	return m.getMonthlyAnalyticsFn(ctx, itemID)
}
func (m *mockStore) CreateCategory(ctx context.Context, name, iconEmoji string) (*models.Category, error) {
	return m.createCategoryFn(ctx, name, iconEmoji)
}
func (m *mockStore) ConsumeItem(ctx context.Context, id int, amount float64, reason string) error {
	return m.consumeItemFn(ctx, id, amount, reason)
}
func (m *mockStore) RestockItem(ctx context.Context, id int, amount float64, pricePaid *float64) error {
	return m.restockItemFn(ctx, id, amount, pricePaid)
}

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return rdb
}

func marshalJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(data)
}

func TestCachedGetStockStatus_CacheHit(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()

	want := []models.StockStatus{
		{ID: 1, Name: "Soap", Status: "ok"},
		{ID: 2, Name: "Shampoo", Status: "low"},
	}
	rdb.Set(ctx, "home-storage:status", marshalJSON(t, want), 30*time.Second)

	dbCalled := false
	store := &mockStore{
		getStockStatusFn: func(ctx context.Context) ([]models.StockStatus, error) {
			dbCalled = true
			return nil, nil
		},
	}

	got, err := repository.NewCachedRepository(store, rdb).GetStockStatus(ctx)
	if err != nil {
		t.Fatalf("GetStockStatus: %v", err)
	}
	if dbCalled {
		t.Error("DB should not be called on a cache hit")
	}
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 2 {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestCachedGetStockStatus_CacheMiss(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()

	want := []models.StockStatus{{ID: 3, Name: "Conditioner", Status: "out"}}

	dbCalled := false
	store := &mockStore{
		getStockStatusFn: func(ctx context.Context) ([]models.StockStatus, error) {
			dbCalled = true
			return want, nil
		},
	}

	got, err := repository.NewCachedRepository(store, rdb).GetStockStatus(ctx)
	if err != nil {
		t.Fatalf("GetStockStatus: %v", err)
	}
	if !dbCalled {
		t.Error("DB should be called on a cache miss")
	}
	if len(got) != 1 || got[0].ID != 3 {
		t.Errorf("unexpected result: %+v", got)
	}

	// Result must be written to cache so the next call is a hit.
	raw, err := rdb.Get(ctx, "home-storage:status").Bytes()
	if err != nil {
		t.Fatalf("expected status key in cache after miss: %v", err)
	}
	var cached []models.StockStatus
	if err = json.Unmarshal(raw, &cached); err != nil {
		t.Fatalf("unmarshal cached value: %v", err)
	}
	if len(cached) != 1 || cached[0].ID != 3 {
		t.Errorf("cached data mismatch: %+v", cached)
	}
}

func TestCachedGetCategories_CacheHit(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()

	want := []models.Category{
		{ID: 1, Name: "Bathroom", IconEmoji: "🛁"},
		{ID: 2, Name: "Pantry", IconEmoji: "🍚"},
	}
	rdb.Set(ctx, "home-storage:categories", marshalJSON(t, want), 24*time.Hour)

	dbCalled := false
	store := &mockStore{
		getCategoriesFn: func(ctx context.Context) ([]models.Category, error) {
			dbCalled = true
			return nil, nil
		},
	}

	got, err := repository.NewCachedRepository(store, rdb).GetCategories(ctx)
	if err != nil {
		t.Fatalf("GetCategories: %v", err)
	}
	if dbCalled {
		t.Error("DB should not be called on a cache hit")
	}
	if len(got) != 2 || got[0].Name != "Bathroom" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestCachedGetCategories_CacheMiss(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()

	want := []models.Category{{ID: 1, Name: "Bathroom", IconEmoji: "🛁"}}

	dbCalled := false
	store := &mockStore{
		getCategoriesFn: func(ctx context.Context) ([]models.Category, error) {
			dbCalled = true
			return want, nil
		},
	}

	got, err := repository.NewCachedRepository(store, rdb).GetCategories(ctx)
	if err != nil {
		t.Fatalf("GetCategories: %v", err)
	}
	if !dbCalled {
		t.Error("DB should be called on a cache miss")
	}
	if len(got) != 1 || got[0].Name != "Bathroom" {
		t.Errorf("unexpected result: %+v", got)
	}

	if _, err = rdb.Get(ctx, "home-storage:categories").Bytes(); err != nil {
		t.Fatalf("expected categories key in cache after miss: %v", err)
	}
}

func TestCachedConsumeItem_InvalidatesStatusCache(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()

	rdb.Set(ctx, "home-storage:status", marshalJSON(t, []models.StockStatus{{ID: 1}}), 30*time.Second)

	store := &mockStore{
		consumeItemFn: func(ctx context.Context, id int, amount float64, reason string) error {
			return nil
		},
	}

	if err := repository.NewCachedRepository(store, rdb).ConsumeItem(ctx, 1, 2.0, "used"); err != nil {
		t.Fatalf("ConsumeItem: %v", err)
	}
	if rdb.Exists(ctx, "home-storage:status").Val() != 0 {
		t.Error("status cache should be invalidated after ConsumeItem")
	}
}

func TestCachedRestockItem_InvalidatesStatusCache(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()

	rdb.Set(ctx, "home-storage:status", marshalJSON(t, []models.StockStatus{{ID: 1}}), 30*time.Second)

	store := &mockStore{
		restockItemFn: func(ctx context.Context, id int, amount float64, pricePaid *float64) error {
			return nil
		},
	}

	if err := repository.NewCachedRepository(store, rdb).RestockItem(ctx, 1, 3.0, nil); err != nil {
		t.Fatalf("RestockItem: %v", err)
	}
	if rdb.Exists(ctx, "home-storage:status").Val() != 0 {
		t.Error("status cache should be invalidated after RestockItem")
	}
}

func TestCachedCreateCategory_InvalidatesCategoriesCache(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()

	rdb.Set(ctx, "home-storage:categories", marshalJSON(t, []models.Category{{ID: 1}}), 24*time.Hour)

	newCat := &models.Category{ID: 5, Name: "Kitchen", IconEmoji: "🍳"}
	store := &mockStore{
		createCategoryFn: func(ctx context.Context, name, iconEmoji string) (*models.Category, error) {
			return newCat, nil
		},
	}

	got, err := repository.NewCachedRepository(store, rdb).CreateCategory(ctx, "Kitchen", "🍳")
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	if got.ID != 5 || got.Name != "Kitchen" {
		t.Errorf("unexpected result: %+v", got)
	}
	if rdb.Exists(ctx, "home-storage:categories").Val() != 0 {
		t.Error("categories cache should be invalidated after CreateCategory")
	}
}

func TestCachedPassThroughMethods(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()
	cat := "Bathroom"
	days := 7.0

	store := &mockStore{
		getLowStockFn: func(ctx context.Context) ([]models.Item, error) {
			return []models.Item{{ID: 1}}, nil
		},
		getItemsFn: func(ctx context.Context) ([]models.ItemWithCategory, error) {
			return []models.ItemWithCategory{{ID: 2, Category: &cat}}, nil
		},
		getItemByIDFn: func(ctx context.Context, id int) (*models.ItemDetail, error) {
			return &models.ItemDetail{
				ItemWithCategory: models.ItemWithCategory{ID: id},
				DaysRemaining:    &days,
			}, nil
		},
		getMonthlyAnalyticsFn: func(ctx context.Context, itemID *int) ([]models.MonthlyConsumption, error) {
			return []models.MonthlyConsumption{{ItemID: 1}}, nil
		},
	}

	repo := repository.NewCachedRepository(store, rdb)

	t.Run("GetLowStock", func(t *testing.T) {
		got, err := repo.GetLowStock(ctx)
		if err != nil || len(got) != 1 || got[0].ID != 1 {
			t.Errorf("GetLowStock: err=%v got=%+v", err, got)
		}
	})

	t.Run("GetItems", func(t *testing.T) {
		got, err := repo.GetItems(ctx)
		if err != nil || len(got) != 1 || got[0].ID != 2 {
			t.Errorf("GetItems: err=%v got=%+v", err, got)
		}
	})

	t.Run("GetItemByID", func(t *testing.T) {
		got, err := repo.GetItemByID(ctx, 42)
		if err != nil || got.ID != 42 || got.DaysRemaining == nil || *got.DaysRemaining != days {
			t.Errorf("GetItemByID: err=%v got=%+v", err, got)
		}
	})

	t.Run("GetMonthlyAnalytics", func(t *testing.T) {
		got, err := repo.GetMonthlyAnalytics(ctx, nil)
		if err != nil || len(got) != 1 || got[0].ItemID != 1 {
			t.Errorf("GetMonthlyAnalytics: err=%v got=%+v", err, got)
		}
	})
}
