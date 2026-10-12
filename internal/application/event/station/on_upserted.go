package station

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"go-api/internal/application/messaging"
	"go-api/internal/domain/event"
	"go-api/internal/domain/port"
	domainstation "go-api/internal/domain/station"

	"github.com/google/uuid"
)

const upsertedHandlerName = "station_upserted"

// UpsertedHandler writes station info from station.upserted.v1.
// Prices are written in the same transaction only when the message creates the station.
type UpsertedHandler struct {
	stations  domainstation.WriteRepository
	outbox    port.OutboxRepository
	processed port.ProcessedEventRepository
	now       func() time.Time
}

func NewUpsertedHandler(stations domainstation.WriteRepository, outbox port.OutboxRepository, processed port.ProcessedEventRepository) *UpsertedHandler {
	return &UpsertedHandler{
		stations:  stations,
		outbox:    outbox,
		processed: processed,
		now:       time.Now,
	}
}

func (h *UpsertedHandler) Handle(ctx context.Context, payload []byte) error {
	var msg domainstation.StationUpserted
	if err := json.Unmarshal(payload, &msg); err != nil {
		return messaging.NonRetryable(err)
	}
	eventID, err := deliveryEventID(ctx, msg.EventID)
	if err != nil {
		return err
	}
	msg.EventID = eventID

	built, err := msg.Station()
	if err != nil {
		return messaging.NonRetryable(err)
	}
	if msg.InfoHash != built.InfoHash() {
		return messaging.NonRetryable(fmt.Errorf("info hash mismatch for station %s", built.StationID))
	}
	if msg.SyncPrices && msg.PriceHash != built.PriceHash() {
		return messaging.NonRetryable(fmt.Errorf("price hash mismatch for station %s", built.StationID))
	}

	observedAt := h.now().UTC()
	var priceChanges int
	skipped := false
	err = h.stations.WithTransaction(ctx, func(txCtx context.Context) error {
		inserted, err := h.processed.MarkProcessed(txCtx, msg.EventID, upsertedHandlerName)
		if err != nil {
			return messaging.Retryable(err)
		}
		if !inserted {
			skipped = true
			return nil
		}

		persistedID, err := h.stations.Upsert(txCtx, built, msg.SyncPrices)
		if err != nil {
			return messaging.Retryable(err)
		}
		if msg.SyncPrices {
			current, err := h.stations.FindPrices(txCtx, persistedID)
			if err != nil {
				return messaging.Retryable(err)
			}
			changes := domainstation.DiffPrices(current, built.Prices)
			if err := h.stations.SyncPrices(txCtx, persistedID, built.Prices, changes, observedAt, built.PriceHash()); err != nil {
				return messaging.Retryable(err)
			}
			events := make([]event.DomainEvent, len(changes))
			for i, change := range changes {
				events[i] = domainstation.NewPriceChanged(persistedID, change, observedAt)
			}
			if err := h.outbox.StoreEvents(txCtx, events); err != nil {
				return messaging.Retryable(err)
			}
			priceChanges = len(changes)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !skipped {
		log.Printf(
			"event handled %s eventId=%s stationId=%s priceChanges=%d",
			domainstation.EventTypeStationUpserted,
			msg.EventID,
			built.StationID,
			priceChanges,
		)
	}
	return nil
}

func deliveryEventID(ctx context.Context, payloadID string) (string, error) {
	if payloadID == "" {
		payloadID = messaging.EventIDFromContext(ctx)
	}
	if _, err := uuid.Parse(payloadID); err != nil {
		return "", messaging.NonRetryable(fmt.Errorf("event id: %w", err))
	}
	return payloadID, nil
}
