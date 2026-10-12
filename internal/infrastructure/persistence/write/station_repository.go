package write

import (
	"context"
	"fmt"
	"time"

	"go-api/internal/domain/price"
	"go-api/internal/domain/station"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type stationWriteRepository struct {
	db *gorm.DB
}

func NewStationWriteRepository(db *gorm.DB) station.WriteRepository {
	return &stationWriteRepository{db: db}
}

func (r *stationWriteRepository) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(ContextWithTx(ctx, tx))
	})
}

func (r *stationWriteRepository) Upsert(ctx context.Context, s station.Station, syncPrices bool) (uuid.UUID, error) {
	model, err := stationModelFromDomain(s, syncPrices)
	if err != nil {
		return uuid.Nil, err
	}
	columns := []string{
		"energy_type",
		"latitude",
		"longitude",
		"postal_code",
		"address",
		"city",
		"pop",
		"automate_24",
		"hours",
		"services",
		"info_hash",
	}
	if syncPrices {
		columns = append(columns, "price_hash")
	}
	err = DBWithContext(ctx, r.db).
		Clauses(
			clause.OnConflict{
				Columns:   []clause.Column{{Name: "station_id"}},
				DoUpdates: clause.AssignmentColumns(columns),
			},
			clause.Returning{Columns: []clause.Column{{Name: "id"}}},
		).
		Create(&model).Error
	if err != nil {
		return uuid.Nil, fmt.Errorf("upsert station %s: %w", s.StationID, err)
	}
	if model.ID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("station %s missing id after upsert", s.StationID)
	}
	return model.ID, nil
}

func (r *stationWriteRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	var count int64
	err := DBWithContext(ctx, r.db).Model(&StationModel{}).Where("id = ?", id).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("lookup station %s: %w", id, err)
	}
	return count > 0, nil
}

type currentPriceRow struct {
	ExternalID      string
	Fuel            string
	Price           euroAmount
	SourceUpdatedAt time.Time
}

func (r *stationWriteRepository) FindPrices(ctx context.Context, id uuid.UUID) ([]price.Price, error) {
	var rows []currentPriceRow
	err := DBWithContext(ctx, r.db).
		Table("current_prices").
		Select("stations.station_id AS external_id, current_prices.fuel, current_prices.price, current_prices.source_updated_at").
		Joins("JOIN stations ON stations.id = current_prices.station_id").
		Where("current_prices.station_id = ?", id).
		Order("current_prices.fuel").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("load prices for station %s: %w", id, err)
	}

	prices := make([]price.Price, len(rows))
	for i, row := range rows {
		prices[i] = price.Price{
			StationID:       row.ExternalID,
			Fuel:            price.Fuel(row.Fuel),
			Amount:          int(row.Price),
			SourceUpdatedAt: row.SourceUpdatedAt.UTC(),
		}
	}
	return prices, nil
}

// SyncPrices replaces the current prices with the snapshot.
// A fuel missing from prices is deleted. History and the caller record only amount changes.
func (r *stationWriteRepository) SyncPrices(ctx context.Context, id uuid.UUID, prices []price.Price, changes []station.PriceChange, observedAt time.Time, priceHash string) error {
	db := DBWithContext(ctx, r.db)
	deleteMissing := db.Where("station_id = ?", id)
	if len(prices) > 0 {
		fuels := make([]string, len(prices))
		for i, item := range prices {
			fuels[i] = string(item.Fuel)
		}
		deleteMissing = deleteMissing.Where("fuel NOT IN ?", fuels)
	}
	if err := deleteMissing.Delete(&CurrentPriceModel{}).Error; err != nil {
		return fmt.Errorf("delete stale prices for station %s: %w", id, err)
	}

	if len(prices) > 0 {
		now := observedAt.UTC()
		rows := make([]CurrentPriceModel, len(prices))
		for i, item := range prices {
			rows[i] = CurrentPriceModel{
				StationID:       id,
				Fuel:            string(item.Fuel),
				Price:           euroAmount(item.Amount),
				SourceUpdatedAt: item.SourceUpdatedAt.UTC(),
				CreatedAt:       now,
				UpdatedAt:       now,
			}
		}
		err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "station_id"}, {Name: "fuel"}},
			DoUpdates: clause.AssignmentColumns([]string{"price", "source_updated_at", "updated_at"}),
		}).Create(&rows).Error
		if err != nil {
			return fmt.Errorf("upsert prices for station %s: %w", id, err)
		}
	}

	updated := db.Model(&StationModel{}).Where("id = ?", id).Update("price_hash", priceHash)
	if updated.Error != nil {
		return fmt.Errorf("store price hash for station %s: %w", id, updated.Error)
	}
	if updated.RowsAffected == 0 {
		return fmt.Errorf("station %s not found", id)
	}
	if len(changes) == 0 {
		return nil
	}
	history := make([]PriceHistoryModel, len(changes))
	observedAt = observedAt.UTC()
	for i, change := range changes {
		history[i] = PriceHistoryModel{
			StationID:       id,
			Fuel:            string(change.Price.Fuel),
			Price:           euroAmount(change.Price.Amount),
			SourceUpdatedAt: change.Price.SourceUpdatedAt.UTC(),
			ObservedAt:      observedAt,
		}
	}
	if err := db.Create(&history).Error; err != nil {
		return fmt.Errorf("insert price history for station %s: %w", id, err)
	}
	return nil
}
