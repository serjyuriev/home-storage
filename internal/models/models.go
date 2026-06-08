package models

import "time"

type Category struct {
	ID        int    `json:"id"         gorm:"column:id;primaryKey"`
	Name      string `json:"name"       gorm:"column:name"`
	IconEmoji string `json:"icon_emoji" gorm:"column:icon_emoji"`
}

func (Category) TableName() string { return "categories" }

type Item struct {
	ID                  int       `json:"id"                    gorm:"column:id;primaryKey"`
	Name                string    `json:"name"                  gorm:"column:name"`
	CategoryID          *int      `json:"category_id"           gorm:"column:category_id"`
	QtyCurrent          float64   `json:"qty_current"           gorm:"column:qty_current"`
	QtyRestockThreshold float64   `json:"qty_restock_threshold" gorm:"column:qty_restock_threshold"`
	Unit                string    `json:"unit"                  gorm:"column:unit"`
	Notes               *string   `json:"notes"                 gorm:"column:notes"`
	UpdatedAt           time.Time `json:"updated_at"            gorm:"column:updated_at"`
}

func (Item) TableName() string { return "items" }

type ItemWithCategory struct {
	ID                  int       `json:"id"                    gorm:"column:id"`
	Name                string    `json:"name"                  gorm:"column:name"`
	CategoryID          *int      `json:"category_id"           gorm:"column:category_id"`
	Category            *string   `json:"category"              gorm:"column:category_name"`
	IconEmoji           *string   `json:"icon_emoji"            gorm:"column:icon_emoji"`
	QtyCurrent          float64   `json:"qty_current"           gorm:"column:qty_current"`
	QtyRestockThreshold float64   `json:"qty_restock_threshold" gorm:"column:qty_restock_threshold"`
	Unit                string    `json:"unit"                  gorm:"column:unit"`
	Notes               *string   `json:"notes"                 gorm:"column:notes"`
	UpdatedAt           time.Time `json:"updated_at"            gorm:"column:updated_at"`
}

type StockStatus struct {
	ID                  int       `json:"id"                    gorm:"column:id"`
	Name                string    `json:"name"                  gorm:"column:name"`
	Category            *string   `json:"category"              gorm:"column:category"`
	IconEmoji           *string   `json:"icon_emoji"            gorm:"column:icon_emoji"`
	QtyCurrent          float64   `json:"qty_current"           gorm:"column:qty_current"`
	QtyRestockThreshold float64   `json:"qty_restock_threshold" gorm:"column:qty_restock_threshold"`
	Unit                string    `json:"unit"                  gorm:"column:unit"`
	DaysRemaining       *float64  `json:"days_remaining"        gorm:"column:days_remaining"`
	Status              string    `json:"status"                gorm:"column:status"`
	Notes               *string   `json:"notes"                 gorm:"column:notes"`
	UpdatedAt           time.Time `json:"updated_at"            gorm:"column:updated_at"`
}

func (StockStatus) TableName() string { return "v_stock_status" }

type MonthlyConsumption struct {
	ItemID         int       `json:"item_id"         gorm:"column:item_id"`
	ItemName       string    `json:"item_name"       gorm:"column:item_name"`
	Category       *string   `json:"category"        gorm:"column:category"`
	Month          time.Time `json:"month"           gorm:"column:month"`
	TotalConsumed  float64   `json:"total_consumed"  gorm:"column:total_consumed"`
	TotalRestocked float64   `json:"total_restocked" gorm:"column:total_restocked"`
	NetChange      float64   `json:"net_change"      gorm:"column:net_change"`
}

func (MonthlyConsumption) TableName() string { return "mv_monthly_consumption" }

type ItemDetail struct {
	ItemWithCategory
	DaysRemaining *float64 `json:"days_remaining"`
}
