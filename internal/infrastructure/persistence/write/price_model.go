package write

import (
	"time"

	"github.com/google/uuid"
)

// CurrentPriceModel is the persistence mapping for table current_prices.
// Schema is owned by SQL migrations — do not encode DDL in tags.
// Price is euros per litre with 3 decimal places.
type CurrentPriceModel struct {
	StationID       uuid.UUID  `gorm:"column:station_id;primaryKey"`
	Fuel            string     `gorm:"column:fuel;primaryKey"`
	Price           euroAmount `gorm:"column:price"`
	SourceUpdatedAt time.Time  `gorm:"column:source_updated_at"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
}

func (CurrentPriceModel) TableName() string {
	return "current_prices"
}

// PriceHistoryModel is the persistence mapping for table prices_history.
// Schema is owned by SQL migrations — do not encode DDL in tags.
type PriceHistoryModel struct {
	ID              int64      `gorm:"column:id;primaryKey"`
	StationID       uuid.UUID  `gorm:"column:station_id"`
	Fuel            string     `gorm:"column:fuel"`
	Price           euroAmount `gorm:"column:price"`
	SourceUpdatedAt time.Time  `gorm:"column:source_updated_at"`
	ObservedAt      time.Time  `gorm:"column:observed_at"`
}

func (PriceHistoryModel) TableName() string {
	return "prices_history"
}
