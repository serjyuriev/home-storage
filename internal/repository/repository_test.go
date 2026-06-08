package repository_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/home-storage/api/internal/config"
	"github.com/home-storage/api/internal/models"
	"github.com/home-storage/api/internal/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	cfg := config.Load()
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort,
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Skipf("skipping integration test — database unavailable: %v", err)
	}
	sqlDB, _ := db.DB()
	if err := sqlDB.Ping(); err != nil {
		t.Skipf("skipping integration test — database unreachable: %v", err)
	}
	return db
}

func testCategory(t *testing.T, db *gorm.DB) models.Category {
	t.Helper()
	c := models.Category{
		Name:      fmt.Sprintf("test_cat_%d", time.Now().UnixNano()),
		IconEmoji: "🧪",
	}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("testCategory: %v", err)
	}
	t.Cleanup(func() { db.Delete(&models.Category{}, c.ID) })
	return c
}

func testItem(t *testing.T, db *gorm.DB, catID *int, name string, qty, threshold float64) models.Item {
	t.Helper()
	item := models.Item{
		Name:                name,
		CategoryID:          catID,
		QtyCurrent:          qty,
		QtyRestockThreshold: threshold,
		Unit:                "pcs",
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("testItem: %v", err)
	}
	t.Cleanup(func() { db.Delete(&models.Item{}, item.ID) })
	return item
}

func TestGetCategories(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDBRepository(db)
	ctx := context.Background()

	a := testCategory(t, db)
	b := testCategory(t, db)

	results, err := repo.GetCategories(ctx)
	if err != nil {
		t.Fatalf("GetCategories: %v", err)
	}

	found := make(map[int]bool, len(results))
	for _, c := range results {
		found[c.ID] = true
	}
	if !found[a.ID] || !found[b.ID] {
		t.Errorf("expected both test categories (ids %d, %d) in results", a.ID, b.ID)
	}
}

func TestGetItems(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDBRepository(db)
	ctx := context.Background()

	cat := testCategory(t, db)
	catID := cat.ID
	item := testItem(t, db, &catID, fmt.Sprintf("test_item_%d", time.Now().UnixNano()), 10, 3)

	results, err := repo.GetItems(ctx)
	if err != nil {
		t.Fatalf("GetItems: %v", err)
	}

	var found *models.ItemWithCategory
	for i := range results {
		if results[i].ID == item.ID {
			found = &results[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("item %d not found in results", item.ID)
	}
	if found.Category == nil || *found.Category != cat.Name {
		t.Errorf("category name: got %v, want %q", found.Category, cat.Name)
	}
}

func TestGetStockStatus(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDBRepository(db)
	ctx := context.Background()

	cat := testCategory(t, db)
	catID := cat.ID
	suffix := time.Now().UnixNano()

	okItem := testItem(t, db, &catID, fmt.Sprintf("test_ok_%d", suffix), 10, 3)
	lowItem := testItem(t, db, &catID, fmt.Sprintf("test_low_%d", suffix), 2, 3)
	outItem := testItem(t, db, &catID, fmt.Sprintf("test_out_%d", suffix), 0, 3)

	results, err := repo.GetStockStatus(ctx)
	if err != nil {
		t.Fatalf("GetStockStatus: %v", err)
	}

	statusByID := make(map[int]string, len(results))
	for _, s := range results {
		statusByID[s.ID] = s.Status
	}

	cases := []struct {
		id   int
		want string
	}{
		{okItem.ID, "ok"},
		{lowItem.ID, "low"},
		{outItem.ID, "out"},
	}
	for _, c := range cases {
		if got := statusByID[c.id]; got != c.want {
			t.Errorf("item %d status: got %q, want %q", c.id, got, c.want)
		}
	}
}

func TestGetLowStock(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDBRepository(db)
	ctx := context.Background()

	cat := testCategory(t, db)
	catID := cat.ID
	suffix := time.Now().UnixNano()

	lowItem := testItem(t, db, &catID, fmt.Sprintf("test_low_%d", suffix), 1, 5)
	okItem := testItem(t, db, &catID, fmt.Sprintf("test_ok_%d", suffix), 10, 5)

	results, err := repo.GetLowStock(ctx)
	if err != nil {
		t.Fatalf("GetLowStock: %v", err)
	}

	ids := make(map[int]bool, len(results))
	for _, it := range results {
		ids[it.ID] = true
	}
	if !ids[lowItem.ID] {
		t.Errorf("expected low item %d in results", lowItem.ID)
	}
	if ids[okItem.ID] {
		t.Errorf("ok item %d should not appear in low-stock results", okItem.ID)
	}
}

func TestGetItemByID(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDBRepository(db)
	ctx := context.Background()

	cat := testCategory(t, db)
	catID := cat.ID

	t.Run("returns item and nil days_remaining when no rate set", func(t *testing.T) {
		item := testItem(t, db, &catID, fmt.Sprintf("test_item_%d", time.Now().UnixNano()), 5, 2)

		got, err := repo.GetItemByID(ctx, item.ID)
		if err != nil {
			t.Fatalf("GetItemByID: %v", err)
		}
		if got.ID != item.ID {
			t.Errorf("id: got %d, want %d", got.ID, item.ID)
		}
		if got.Category == nil || *got.Category != cat.Name {
			t.Errorf("category: got %v, want %q", got.Category, cat.Name)
		}
		if got.DaysRemaining != nil {
			t.Errorf("days_remaining: expected nil (no rate), got %.2f", *got.DaysRemaining)
		}
	})

	t.Run("returns computed days_remaining when consumption rate is set", func(t *testing.T) {
		item := testItem(t, db, &catID, fmt.Sprintf("test_item_rate_%d", time.Now().UnixNano()), 7, 2)
		// Set composite type via raw SQL — GORM cannot write consumption_rate directly.
		// ROW(1, '1 day') means consume 1 unit per day.
		if err := db.Exec(
			"UPDATE items SET rate = ROW(1, '1 day'::interval) WHERE id = ?", item.ID,
		).Error; err != nil {
			t.Fatalf("set rate: %v", err)
		}

		got, err := repo.GetItemByID(ctx, item.ID)
		if err != nil {
			t.Fatalf("GetItemByID: %v", err)
		}
		if got.DaysRemaining == nil {
			t.Fatal("days_remaining: expected non-nil (rate is set)")
		}
		// qty=7, rate=1/day → estimate_days_remaining returns ROUND(7/1, 1) = 7.0
		if *got.DaysRemaining != 7.0 {
			t.Errorf("days_remaining: got %.1f, want 7.0", *got.DaysRemaining)
		}
	})

	t.Run("non-existent id returns ErrNotFound", func(t *testing.T) {
		_, err := repo.GetItemByID(ctx, -1)
		if !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("error: got %v, want ErrNotFound", err)
		}
	})
}

func TestGetMonthlyAnalytics(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDBRepository(db)
	ctx := context.Background()

	// Set up fixtures manually to control the cleanup order precisely:
	// stock_history must be deleted before refreshing the view for a clean state.
	cat := models.Category{
		Name:      fmt.Sprintf("test_cat_%d", time.Now().UnixNano()),
		IconEmoji: "🧪",
	}
	if err := db.Create(&cat).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}
	item := models.Item{
		Name:                fmt.Sprintf("test_analytics_%d", time.Now().UnixNano()),
		CategoryID:          &cat.ID,
		QtyCurrent:          10,
		QtyRestockThreshold: 2,
		Unit:                "pcs",
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create item: %v", err)
	}

	// Insert a stock_history row directly. The trigger only fires on UPDATE of
	// items.qty_current, so this INSERT does not create a duplicate entry.
	// change_amount=-2 → view computes total_consumed = (-(-2)) = 2.
	firstOfMonth := time.Now().UTC().AddDate(0, 0, -(time.Now().UTC().Day() - 1)).Truncate(24 * time.Hour)
	if err := db.Exec(`
		INSERT INTO stock_history (item_id, change_amount, qty_after, reason, changed_at)
		VALUES (?, ?, ?, ?, ?)
	`, item.ID, -2.0, 8.0, "test_usage", firstOfMonth).Error; err != nil {
		t.Fatalf("insert stock_history: %v", err)
	}

	if err := db.Exec("REFRESH MATERIALIZED VIEW mv_monthly_consumption").Error; err != nil {
		t.Fatalf("refresh materialized view: %v", err)
	}

	// Cleanup in the correct order: history → item → refresh (view now clean) → category.
	t.Cleanup(func() {
		db.Exec("DELETE FROM stock_history WHERE item_id = ?", item.ID)
		db.Exec("DELETE FROM items WHERE id = ?", item.ID)
		db.Exec("REFRESH MATERIALIZED VIEW mv_monthly_consumption")
		db.Exec("DELETE FROM categories WHERE id = ?", cat.ID)
	})

	t.Run("no filter includes our test item", func(t *testing.T) {
		results, err := repo.GetMonthlyAnalytics(ctx, nil)
		if err != nil {
			t.Fatalf("GetMonthlyAnalytics(nil): %v", err)
		}
		var found *models.MonthlyConsumption
		for i := range results {
			if results[i].ItemID == item.ID {
				found = &results[i]
				break
			}
		}
		if found == nil {
			t.Fatalf("item %d not found in monthly analytics results", item.ID)
		}
		// SUM(change_amount < 0) * -1 = (-(-2)) = 2
		if found.TotalConsumed != 2.0 {
			t.Errorf("total_consumed: got %v, want 2.0", found.TotalConsumed)
		}
		if found.NetChange != -2.0 {
			t.Errorf("net_change: got %v, want -2.0", found.NetChange)
		}
	})

	t.Run("item_id filter returns only rows for that item", func(t *testing.T) {
		results, err := repo.GetMonthlyAnalytics(ctx, &item.ID)
		if err != nil {
			t.Fatalf("GetMonthlyAnalytics(itemID): %v", err)
		}
		if len(results) == 0 {
			t.Fatal("expected at least one row")
		}
		for _, r := range results {
			if r.ItemID != item.ID {
				t.Errorf("unexpected item_id %d in filtered results", r.ItemID)
			}
		}
	})
}

func TestCreateCategory(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDBRepository(db)
	ctx := context.Background()

	t.Run("creates category and returns it with assigned id", func(t *testing.T) {
		name := fmt.Sprintf("test_create_%d", time.Now().UnixNano())
		got, err := repo.CreateCategory(ctx, name, "🧪")
		if err != nil {
			t.Fatalf("CreateCategory: %v", err)
		}
		if got.ID == 0 {
			t.Error("expected non-zero id after create")
		}
		if got.Name != name || got.IconEmoji != "🧪" {
			t.Errorf("unexpected result: %+v", got)
		}
		t.Cleanup(func() { db.Delete(&models.Category{}, got.ID) })
	})

	t.Run("returns ErrDuplicate for existing name", func(t *testing.T) {
		// "Bathroom" is one of the seed categories defined in database.sql.
		_, err := repo.CreateCategory(ctx, "Bathroom", "")
		if !errors.Is(err, repository.ErrDuplicate) {
			t.Errorf("expected ErrDuplicate, got %v", err)
		}
	})
}

func TestConsumeItem(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDBRepository(db)
	ctx := context.Background()

	cat := testCategory(t, db)
	catID := cat.ID

	t.Run("decrements qty_current by the consumed amount", func(t *testing.T) {
		item := testItem(t, db, &catID, fmt.Sprintf("test_consume_%d", time.Now().UnixNano()), 5, 2)
		// Cleanup stock_history created by the trigger when log_usage updates qty_current.
		t.Cleanup(func() { db.Exec("DELETE FROM stock_history WHERE item_id = ?", item.ID) })

		if err := repo.ConsumeItem(ctx, item.ID, 2, "test usage"); err != nil {
			t.Fatalf("ConsumeItem: %v", err)
		}

		var updated models.Item
		db.Raw("SELECT id, qty_current FROM items WHERE id = ?", item.ID).Scan(&updated)
		if updated.QtyCurrent != 3 {
			t.Errorf("qty_current: got %v, want 3", updated.QtyCurrent)
		}
	})

	t.Run("returns ErrInsufficientStock when amount exceeds qty_current", func(t *testing.T) {
		item := testItem(t, db, &catID, fmt.Sprintf("test_low_%d", time.Now().UnixNano()), 1, 0)

		err := repo.ConsumeItem(ctx, item.ID, 5, "test")
		if !errors.Is(err, repository.ErrInsufficientStock) {
			t.Errorf("expected ErrInsufficientStock, got %v", err)
		}
		// Procedure raises exception before modifying anything — no history rows to clean up.
	})

	t.Run("returns ErrNotFound for non-existent item", func(t *testing.T) {
		err := repo.ConsumeItem(ctx, -1, 1, "test")
		if !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestRestockItem(t *testing.T) {
	db := openTestDB(t)
	repo := repository.NewDBRepository(db)
	ctx := context.Background()

	cat := testCategory(t, db)
	catID := cat.ID

	t.Run("increments qty_current by the restocked amount", func(t *testing.T) {
		item := testItem(t, db, &catID, fmt.Sprintf("test_restock_%d", time.Now().UnixNano()), 5, 2)
		// restock_item procedure creates 2 history entries: one from the UPDATE trigger
		// (reason=NULL) and one inserted manually by the procedure (reason="restock").
		t.Cleanup(func() { db.Exec("DELETE FROM stock_history WHERE item_id = ?", item.ID) })

		if err := repo.RestockItem(ctx, item.ID, 3, nil); err != nil {
			t.Fatalf("RestockItem: %v", err)
		}

		var updated models.Item
		db.Raw("SELECT id, qty_current FROM items WHERE id = ?", item.ID).Scan(&updated)
		if updated.QtyCurrent != 8 {
			t.Errorf("qty_current: got %v, want 8", updated.QtyCurrent)
		}
	})

	t.Run("records price_paid in history reason when provided", func(t *testing.T) {
		item := testItem(t, db, &catID, fmt.Sprintf("test_restock_price_%d", time.Now().UnixNano()), 5, 2)
		t.Cleanup(func() { db.Exec("DELETE FROM stock_history WHERE item_id = ?", item.ID) })

		price := 9.99
		if err := repo.RestockItem(ctx, item.ID, 2, &price); err != nil {
			t.Fatalf("RestockItem with price: %v", err)
		}

		// The procedure inserts a history row with reason = "restock (paid: 9.99)".
		var reason string
		db.Raw(
			"SELECT reason FROM stock_history WHERE item_id = ? AND reason IS NOT NULL LIMIT 1",
			item.ID,
		).Scan(&reason)
		if !strings.Contains(reason, "9.99") {
			t.Errorf("history reason: got %q, expected it to contain '9.99'", reason)
		}
	})

	t.Run("returns ErrNotFound for non-existent item", func(t *testing.T) {
		err := repo.RestockItem(ctx, -1, 1, nil)
		if !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}
