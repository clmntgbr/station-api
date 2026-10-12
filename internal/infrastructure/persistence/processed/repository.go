package processed

import (
	"context"
	"time"

	"go-api/internal/domain/port"
	"go-api/internal/infrastructure/persistence/write"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) port.ProcessedEventRepository {
	return &repository{db: db}
}

func (r *repository) MarkProcessed(ctx context.Context, eventID, handlerName string) (bool, error) {
	id, err := uuid.Parse(eventID)
	if err != nil {
		return false, err
	}

	row := ProcessedEvent{
		EventID:     id,
		HandlerName: handlerName,
		ProcessedAt: time.Now().UTC(),
	}

	// ON CONFLICT DO NOTHING leaves the surrounding transaction usable.
	result := write.DBWithContext(ctx, r.db).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "event_id"}, {Name: "handler_name"}},
			DoNothing: true,
		}).
		Create(&row)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *repository) UnmarkProcessed(ctx context.Context, eventID, handlerName string) error {
	id, err := uuid.Parse(eventID)
	if err != nil {
		return err
	}
	return write.DBWithContext(ctx, r.db).
		Where("event_id = ? AND handler_name = ?", id, handlerName).
		Delete(&ProcessedEvent{}).Error
}
