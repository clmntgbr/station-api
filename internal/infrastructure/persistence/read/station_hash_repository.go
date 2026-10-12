package read

import (
	"context"
	"fmt"

	"go-api/internal/domain/station"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type stationHashRow struct {
	ID        uuid.UUID `gorm:"column:id"`
	StationID string    `gorm:"column:station_id"`
	InfoHash  string    `gorm:"column:info_hash"`
	PriceHash string    `gorm:"column:price_hash"`
}

func (stationHashRow) TableName() string { return "stations" }

type stationHashRepository struct {
	db *gorm.DB
}

func NewStationHashRepository(db *gorm.DB) station.HashReader {
	return &stationHashRepository{db: db}
}

func (r *stationHashRepository) FindHashes(ctx context.Context) (map[string]station.Hashes, error) {
	var rows []stationHashRow
	err := r.db.WithContext(ctx).Select("id", "station_id", "info_hash", "price_hash").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("load station hashes: %w", err)
	}

	hashes := make(map[string]station.Hashes, len(rows))
	for _, row := range rows {
		hashes[row.StationID] = station.Hashes{ID: row.ID, Info: row.InfoHash, Price: row.PriceHash}
	}
	return hashes, nil
}
