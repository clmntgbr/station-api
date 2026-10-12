package read

import (
	"context"
	"fmt"

	"go-api/internal/domain/station"

	"gorm.io/gorm"
)

type stationHashRow struct {
	ID          string
	ContentHash string
}

func (stationHashRow) TableName() string { return "stations" }

type stationHashRepository struct {
	db *gorm.DB
}

func NewStationHashRepository(db *gorm.DB) station.ContentHashReader {
	return &stationHashRepository{db: db}
}

func (r *stationHashRepository) FindContentHashes(ctx context.Context) (map[string]string, error) {
	var rows []stationHashRow
	err := r.db.WithContext(ctx).Select("id", "content_hash").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("load station hashes: %w", err)
	}

	hashes := make(map[string]string, len(rows))
	for _, row := range rows {
		hashes[row.ID] = row.ContentHash
	}
	return hashes, nil
}
