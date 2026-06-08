package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/home-storage/api/internal/handler"
	"github.com/home-storage/api/internal/models"
	"github.com/home-storage/api/internal/repository"
	"github.com/home-storage/api/internal/router"
)

func bodyJSON(v any) *bytes.Buffer {
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

type MockRepository struct {
	GetStockStatusFn      func(ctx context.Context) ([]models.StockStatus, error)
	GetLowStockFn         func(ctx context.Context) ([]models.Item, error)
	GetCategoriesFn       func(ctx context.Context) ([]models.Category, error)
	GetItemsFn            func(ctx context.Context) ([]models.ItemWithCategory, error)
	GetItemByIDFn         func(ctx context.Context, id int) (*models.ItemDetail, error)
	GetMonthlyAnalyticsFn func(ctx context.Context, itemID *int) ([]models.MonthlyConsumption, error)
	CreateCategoryFn      func(ctx context.Context, name, iconEmoji string) (*models.Category, error)
	ConsumeItemFn         func(ctx context.Context, id int, amount float64, reason string) error
	RestockItemFn         func(ctx context.Context, id int, amount float64, pricePaid *float64) error
}

func (m *MockRepository) GetStockStatus(ctx context.Context) ([]models.StockStatus, error) {
	return m.GetStockStatusFn(ctx)
}
func (m *MockRepository) GetLowStock(ctx context.Context) ([]models.Item, error) {
	return m.GetLowStockFn(ctx)
}
func (m *MockRepository) GetCategories(ctx context.Context) ([]models.Category, error) {
	return m.GetCategoriesFn(ctx)
}
func (m *MockRepository) GetItems(ctx context.Context) ([]models.ItemWithCategory, error) {
	return m.GetItemsFn(ctx)
}
func (m *MockRepository) GetItemByID(ctx context.Context, id int) (*models.ItemDetail, error) {
	return m.GetItemByIDFn(ctx, id)
}
func (m *MockRepository) GetMonthlyAnalytics(ctx context.Context, itemID *int) ([]models.MonthlyConsumption, error) {
	return m.GetMonthlyAnalyticsFn(ctx, itemID)
}
func (m *MockRepository) CreateCategory(ctx context.Context, name, iconEmoji string) (*models.Category, error) {
	return m.CreateCategoryFn(ctx, name, iconEmoji)
}
func (m *MockRepository) ConsumeItem(ctx context.Context, id int, amount float64, reason string) error {
	return m.ConsumeItemFn(ctx, id, amount, reason)
}
func (m *MockRepository) RestockItem(ctx context.Context, id int, amount float64, pricePaid *float64) error {
	return m.RestockItemFn(ctx, id, amount, pricePaid)
}

func newTestRouter(mock *MockRepository) *gin.Engine {
	return router.Setup(handler.New(mock))
}

func TestGetStatus(t *testing.T) {
	now := time.Now()
	cat := "Bathroom"

	tests := []struct {
		name       string
		mockFn     func(ctx context.Context) ([]models.StockStatus, error)
		wantStatus int
		wantCount  int
	}{
		{
			name: "returns full list ordered by status",
			mockFn: func(ctx context.Context) ([]models.StockStatus, error) {
				return []models.StockStatus{
					{ID: 1, Name: "Soap", Category: &cat, Status: "out", UpdatedAt: now},
					{ID: 2, Name: "Shampoo", Category: &cat, Status: "ok", UpdatedAt: now},
				}, nil
			},
			wantStatus: http.StatusOK,
			wantCount:  2,
		},
		{
			name:       "empty inventory returns empty array",
			mockFn:     func(ctx context.Context) ([]models.StockStatus, error) { return []models.StockStatus{}, nil },
			wantStatus: http.StatusOK,
			wantCount:  0,
		},
		{
			name:       "db error returns 500",
			mockFn:     func(ctx context.Context) ([]models.StockStatus, error) { return nil, errors.New("connection lost") },
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRouter(&MockRepository{GetStockStatusFn: tt.mockFn})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/status", nil))

			if w.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusOK {
				var body []models.StockStatus
				mustUnmarshal(t, w.Body.Bytes(), &body)
				if len(body) != tt.wantCount {
					t.Errorf("item count: got %d, want %d", len(body), tt.wantCount)
				}
			}
		})
	}
}

func TestGetLowStock(t *testing.T) {
	tests := []struct {
		name       string
		mockFn     func(ctx context.Context) ([]models.Item, error)
		wantStatus int
		wantCount  int
	}{
		{
			name: "returns low-stock items",
			mockFn: func(ctx context.Context) ([]models.Item, error) {
				return []models.Item{{ID: 3, Name: "Toothpaste", QtyCurrent: 1, QtyRestockThreshold: 2}}, nil
			},
			wantStatus: http.StatusOK,
			wantCount:  1,
		},
		{
			name:       "nothing low returns empty array",
			mockFn:     func(ctx context.Context) ([]models.Item, error) { return []models.Item{}, nil },
			wantStatus: http.StatusOK,
			wantCount:  0,
		},
		{
			name:       "db error returns 500",
			mockFn:     func(ctx context.Context) ([]models.Item, error) { return nil, errors.New("timeout") },
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRouter(&MockRepository{GetLowStockFn: tt.mockFn})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/status/low", nil))

			if w.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusOK {
				var body []models.Item
				mustUnmarshal(t, w.Body.Bytes(), &body)
				if len(body) != tt.wantCount {
					t.Errorf("item count: got %d, want %d", len(body), tt.wantCount)
				}
			}
		})
	}
}

func TestGetCategories(t *testing.T) {
	tests := []struct {
		name       string
		mockFn     func(ctx context.Context) ([]models.Category, error)
		wantStatus int
		wantCount  int
	}{
		{
			name: "returns all categories",
			mockFn: func(ctx context.Context) ([]models.Category, error) {
				return []models.Category{
					{ID: 1, Name: "Bathroom", IconEmoji: "🛁"},
					{ID: 2, Name: "Pantry", IconEmoji: "🍚"},
				}, nil
			},
			wantStatus: http.StatusOK,
			wantCount:  2,
		},
		{
			name:       "db error returns 500",
			mockFn:     func(ctx context.Context) ([]models.Category, error) { return nil, errors.New("db error") },
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRouter(&MockRepository{GetCategoriesFn: tt.mockFn})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil))

			if w.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusOK {
				var body []models.Category
				mustUnmarshal(t, w.Body.Bytes(), &body)
				if len(body) != tt.wantCount {
					t.Errorf("item count: got %d, want %d", len(body), tt.wantCount)
				}
			}
		})
	}
}

func TestGetItems(t *testing.T) {
	cat := "Bathroom"

	tests := []struct {
		name       string
		mockFn     func(ctx context.Context) ([]models.ItemWithCategory, error)
		wantStatus int
		wantCount  int
	}{
		{
			name: "returns items with category names",
			mockFn: func(ctx context.Context) ([]models.ItemWithCategory, error) {
				return []models.ItemWithCategory{
					{ID: 1, Name: "Soap", Category: &cat, QtyCurrent: 5, Unit: "pcs"},
				}, nil
			},
			wantStatus: http.StatusOK,
			wantCount:  1,
		},
		{
			name:       "db error returns 500",
			mockFn:     func(ctx context.Context) ([]models.ItemWithCategory, error) { return nil, errors.New("db error") },
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRouter(&MockRepository{GetItemsFn: tt.mockFn})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/items", nil))

			if w.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusOK {
				var body []models.ItemWithCategory
				mustUnmarshal(t, w.Body.Bytes(), &body)
				if len(body) != tt.wantCount {
					t.Errorf("item count: got %d, want %d", len(body), tt.wantCount)
				}
			}
		})
	}
}

func TestGetItemByID(t *testing.T) {
	cat := "Bathroom"
	days := 14.5

	tests := []struct {
		name       string
		urlID      string
		mockFn     func(ctx context.Context, id int) (*models.ItemDetail, error)
		wantStatus int
		checkBody  func(t *testing.T, body []byte)
	}{
		{
			name:  "returns item with days_remaining",
			urlID: "1",
			mockFn: func(ctx context.Context, id int) (*models.ItemDetail, error) {
				if id != 1 {
					t.Errorf("unexpected id passed to repo: %d", id)
				}
				return &models.ItemDetail{
					ItemWithCategory: models.ItemWithCategory{ID: 1, Name: "Soap", Category: &cat},
					DaysRemaining:    &days,
				}, nil
			},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body []byte) {
				var got models.ItemDetail
				mustUnmarshal(t, body, &got)
				if got.ID != 1 {
					t.Errorf("id: got %d, want 1", got.ID)
				}
				if got.DaysRemaining == nil || *got.DaysRemaining != days {
					t.Errorf("days_remaining: got %v, want %v", got.DaysRemaining, days)
				}
			},
		},
		{
			name:  "returns item with nil days_remaining when rate not set",
			urlID: "2",
			mockFn: func(ctx context.Context, id int) (*models.ItemDetail, error) {
				return &models.ItemDetail{
					ItemWithCategory: models.ItemWithCategory{ID: 2, Name: "Toothpaste"},
					DaysRemaining:    nil,
				}, nil
			},
			wantStatus: http.StatusOK,
			checkBody: func(t *testing.T, body []byte) {
				var got models.ItemDetail
				mustUnmarshal(t, body, &got)
				if got.DaysRemaining != nil {
					t.Errorf("expected nil days_remaining, got %v", *got.DaysRemaining)
				}
			},
		},
		{
			name:       "non-existent id returns 404",
			urlID:      "999",
			mockFn:     func(ctx context.Context, id int) (*models.ItemDetail, error) { return nil, repository.ErrNotFound },
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "non-numeric id returns 400",
			urlID:      "abc",
			mockFn:     nil, // should not be called
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "db error returns 500",
			urlID:      "1",
			mockFn:     func(ctx context.Context, id int) (*models.ItemDetail, error) { return nil, errors.New("db error") },
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockRepository{}
			if tt.mockFn != nil {
				mock.GetItemByIDFn = tt.mockFn
			} else {
				mock.GetItemByIDFn = func(ctx context.Context, id int) (*models.ItemDetail, error) {
					t.Error("repository should not have been called")
					return nil, nil
				}
			}

			r := newTestRouter(mock)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/items/"+tt.urlID, nil))

			if w.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.checkBody != nil && tt.wantStatus == http.StatusOK {
				tt.checkBody(t, w.Body.Bytes())
			}
		})
	}
}

func TestGetMonthlyAnalytics(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		mockFn     func(ctx context.Context, itemID *int) ([]models.MonthlyConsumption, error)
		wantStatus int
		wantCount  int
	}{
		{
			name:  "no filter returns all rows",
			query: "",
			mockFn: func(ctx context.Context, itemID *int) ([]models.MonthlyConsumption, error) {
				if itemID != nil {
					t.Errorf("expected nil itemID, got %d", *itemID)
				}
				return []models.MonthlyConsumption{
					{ItemID: 1, ItemName: "Soap", TotalConsumed: 3},
					{ItemID: 2, ItemName: "Shampoo", TotalConsumed: 1},
				}, nil
			},
			wantStatus: http.StatusOK,
			wantCount:  2,
		},
		{
			name:  "item_id filter passes value to repository",
			query: "?item_id=1",
			mockFn: func(ctx context.Context, itemID *int) ([]models.MonthlyConsumption, error) {
				if itemID == nil || *itemID != 1 {
					t.Errorf("expected itemID=1, got %v", itemID)
				}
				return []models.MonthlyConsumption{
					{ItemID: 1, ItemName: "Soap", TotalConsumed: 3},
				}, nil
			},
			wantStatus: http.StatusOK,
			wantCount:  1,
		},
		{
			name:       "non-numeric item_id returns 400",
			query:      "?item_id=bad",
			mockFn:     nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:  "db error returns 500",
			query: "",
			mockFn: func(ctx context.Context, itemID *int) ([]models.MonthlyConsumption, error) {
				return nil, errors.New("db error")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockRepository{}
			if tt.mockFn != nil {
				mock.GetMonthlyAnalyticsFn = tt.mockFn
			} else {
				mock.GetMonthlyAnalyticsFn = func(ctx context.Context, itemID *int) ([]models.MonthlyConsumption, error) {
					t.Error("repository should not have been called")
					return nil, nil
				}
			}

			r := newTestRouter(mock)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/analytics/monthly"+tt.query, nil))

			if w.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusOK {
				var body []models.MonthlyConsumption
				mustUnmarshal(t, w.Body.Bytes(), &body)
				if len(body) != tt.wantCount {
					t.Errorf("row count: got %d, want %d", len(body), tt.wantCount)
				}
			}
		})
	}
}

func TestCreateCategory(t *testing.T) {
	tests := []struct {
		name       string
		body       any
		mockFn     func(ctx context.Context, name, iconEmoji string) (*models.Category, error)
		wantStatus int
		checkBody  func(t *testing.T, body []byte)
	}{
		{
			name: "creates category and returns 201",
			body: map[string]any{"name": "Kitchen", "icon_emoji": "🍳"},
			mockFn: func(ctx context.Context, name, iconEmoji string) (*models.Category, error) {
				if name != "Kitchen" {
					t.Errorf("name: got %q, want Kitchen", name)
				}
				if iconEmoji != "🍳" {
					t.Errorf("icon_emoji: got %q, want 🍳", iconEmoji)
				}
				return &models.Category{ID: 5, Name: name, IconEmoji: iconEmoji}, nil
			},
			wantStatus: http.StatusCreated,
			checkBody: func(t *testing.T, body []byte) {
				var got models.Category
				mustUnmarshal(t, body, &got)
				if got.ID != 5 || got.Name != "Kitchen" {
					t.Errorf("unexpected body: %+v", got)
				}
			},
		},
		{
			name:       "missing name returns 400",
			body:       map[string]any{"icon_emoji": "🍳"},
			mockFn:     nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "duplicate name returns 409",
			body: map[string]any{"name": "Bathroom"},
			mockFn: func(ctx context.Context, name, iconEmoji string) (*models.Category, error) {
				return nil, repository.ErrDuplicate
			},
			wantStatus: http.StatusConflict,
		},
		{
			name: "db error returns 500",
			body: map[string]any{"name": "Kitchen"},
			mockFn: func(ctx context.Context, name, iconEmoji string) (*models.Category, error) {
				return nil, errors.New("db error")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockRepository{}
			if tt.mockFn != nil {
				mock.CreateCategoryFn = tt.mockFn
			} else {
				mock.CreateCategoryFn = func(ctx context.Context, name, iconEmoji string) (*models.Category, error) {
					t.Error("repository should not have been called")
					return nil, nil
				}
			}

			r := newTestRouter(mock)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/categories", bodyJSON(tt.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d\nbody: %s", w.Code, tt.wantStatus, w.Body)
			}
			if tt.checkBody != nil {
				tt.checkBody(t, w.Body.Bytes())
			}
		})
	}
}

func TestConsumeItem(t *testing.T) {
	tests := []struct {
		name       string
		urlID      string
		body       any
		mockFn     func(ctx context.Context, id int, amount float64, reason string) error
		wantStatus int
	}{
		{
			name:  "valid consume returns 204",
			urlID: "1",
			body:  map[string]any{"amount": 2.0, "reason": "weekly use"},
			mockFn: func(ctx context.Context, id int, amount float64, reason string) error {
				if id != 1 || amount != 2.0 || reason != "weekly use" {
					t.Errorf("unexpected args: id=%d amount=%v reason=%q", id, amount, reason)
				}
				return nil
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name:  "missing reason defaults to 'used'",
			urlID: "1",
			body:  map[string]any{"amount": 1.0},
			mockFn: func(ctx context.Context, id int, amount float64, reason string) error {
				if reason != "used" {
					t.Errorf("reason: got %q, want 'used'", reason)
				}
				return nil
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "non-numeric id returns 400",
			urlID:      "abc",
			body:       map[string]any{"amount": 1.0},
			mockFn:     nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "zero amount returns 400",
			urlID:      "1",
			body:       map[string]any{"amount": 0},
			mockFn:     nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "negative amount returns 400",
			urlID:      "1",
			body:       map[string]any{"amount": -3.0},
			mockFn:     nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "item not found returns 404",
			urlID:      "99",
			body:       map[string]any{"amount": 1.0},
			mockFn:     func(ctx context.Context, id int, amount float64, reason string) error { return repository.ErrNotFound },
			wantStatus: http.StatusNotFound,
		},
		{
			name:  "insufficient stock returns 422",
			urlID: "1",
			body:  map[string]any{"amount": 999.0},
			mockFn: func(ctx context.Context, id int, amount float64, reason string) error {
				return repository.ErrInsufficientStock
			},
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name:       "db error returns 500",
			urlID:      "1",
			body:       map[string]any{"amount": 1.0},
			mockFn:     func(ctx context.Context, id int, amount float64, reason string) error { return errors.New("db error") },
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockRepository{}
			if tt.mockFn != nil {
				mock.ConsumeItemFn = tt.mockFn
			} else {
				mock.ConsumeItemFn = func(ctx context.Context, id int, amount float64, reason string) error {
					t.Error("repository should not have been called")
					return nil
				}
			}

			r := newTestRouter(mock)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/items/"+tt.urlID+"/consume", bodyJSON(tt.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d\nbody: %s", w.Code, tt.wantStatus, w.Body)
			}
		})
	}
}

func TestRestockItem(t *testing.T) {
	price := 4.99

	tests := []struct {
		name       string
		urlID      string
		body       any
		mockFn     func(ctx context.Context, id int, amount float64, pricePaid *float64) error
		wantStatus int
	}{
		{
			name:  "valid restock without price returns 204",
			urlID: "2",
			body:  map[string]any{"amount": 5.0},
			mockFn: func(ctx context.Context, id int, amount float64, pricePaid *float64) error {
				if id != 2 || amount != 5.0 || pricePaid != nil {
					t.Errorf("unexpected args: id=%d amount=%v pricePaid=%v", id, amount, pricePaid)
				}
				return nil
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name:  "valid restock with price passes price to repo",
			urlID: "2",
			body:  map[string]any{"amount": 3.0, "price_paid": price},
			mockFn: func(ctx context.Context, id int, amount float64, pricePaid *float64) error {
				if pricePaid == nil || *pricePaid != price {
					t.Errorf("price_paid: got %v, want %v", pricePaid, price)
				}
				return nil
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "non-numeric id returns 400",
			urlID:      "xyz",
			body:       map[string]any{"amount": 1.0},
			mockFn:     nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "zero amount returns 400",
			urlID:      "2",
			body:       map[string]any{"amount": 0},
			mockFn:     nil,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:  "item not found returns 404",
			urlID: "99",
			body:  map[string]any{"amount": 1.0},
			mockFn: func(ctx context.Context, id int, amount float64, pricePaid *float64) error {
				return repository.ErrNotFound
			},
			wantStatus: http.StatusNotFound,
		},
		{
			name:  "db error returns 500",
			urlID: "2",
			body:  map[string]any{"amount": 1.0},
			mockFn: func(ctx context.Context, id int, amount float64, pricePaid *float64) error {
				return errors.New("db error")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &MockRepository{}
			if tt.mockFn != nil {
				mock.RestockItemFn = tt.mockFn
			} else {
				mock.RestockItemFn = func(ctx context.Context, id int, amount float64, pricePaid *float64) error {
					t.Error("repository should not have been called")
					return nil
				}
			}

			r := newTestRouter(mock)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/items/"+tt.urlID+"/restock", bodyJSON(tt.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status: got %d, want %d\nbody: %s", w.Code, tt.wantStatus, w.Body)
			}
		})
	}
}

func mustUnmarshal(t *testing.T, data []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("failed to unmarshal response body: %v\nbody: %s", err, data)
	}
}
