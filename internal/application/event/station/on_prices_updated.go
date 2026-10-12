package station

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"go-api/internal/application/messaging"
	"go-api/internal/domain/event"
	domainstation "go-api/internal/domain/station"
)

const pricesUpdatedHandlerName = "station_prices_updated"

// PricesUpdatedHandler writes prices from station.prices_updated.v1.
// The station row must already exist. A missing station is retried, so a price
// task that arrives before creation does not get discarded.
type PricesUpdatedHandler struct {
	stations  domainstation.WriteRepository
	outbox    eventOutbox
	processed eventProcessor
	now       func() time.Time
}

type eventOutbox interface {
	StoreEvents(ctx context.Context, events []event.DomainEvent) error
}

type eventProcessor interface {
	MarkProcessed(ctx context.Context, eventID, handlerName string) (bool, error)
}

func NewPricesUpdatedHandler(stations domainstation.WriteRepository, outbox eventOutbox, processed eventProcessor) *PricesUpdatedHandler {
	return &PricesUpdatedHandler{
		stations:  stations,
		outbox:    outbox,
		processed: processed,
		now:       time.Now,
	}
}

func (h *PricesUpdatedHandler) Handle(ctx context.Context, payload []byte) error {
	var msg domainstation.PricesUpdated
	if err := json.Unmarshal(payload, &msg); err != nil {
		return messaging.NonRetryable(err)
	}
	eventID, err := deliveryEventID(ctx, msg.EventID)
	if err != nil {
		return err
	}
	msg.EventID = eventID

	priced, err := msg.StationPrices()
	if err != nil {
		return messaging.NonRetryable(err)
	}
	if msg.PriceHash != priced.PriceHash() {
		return messaging.NonRetryable(fmt.Errorf("price hash mismatch for station %s", priced.StationID))
	}

	exists, err := h.stations.Exists(ctx, priced.ID)
	if err != nil {
		return messaging.Retryable(err)
	}
	if !exists {
		return messaging.Retryable(fmt.Errorf("station %s is not created yet", priced.StationID))
	}

	observedAt := h.now().UTC()
	var priceChanges int
	skipped := false
	err = h.stations.WithTransaction(ctx, func(txCtx context.Context) error {
		inserted, err := h.processed.MarkProcessed(txCtx, msg.EventID, pricesUpdatedHandlerName)
		if err != nil {
			return messaging.Retryable(err)
		}
		if !inserted {
			skipped = true
			return nil
		}
		current, err := h.stations.FindPrices(txCtx, priced.ID)
		if err != nil {
			return messaging.Retryable(err)
		}
		changes := domainstation.DiffPrices(current, priced.Prices)
		if err := h.stations.SyncPrices(txCtx, priced.ID, priced.Prices, changes, observedAt, priced.PriceHash()); err != nil {
			return messaging.Retryable(err)
		}
		events := make([]event.DomainEvent, len(changes))
		for i, change := range changes {
			events[i] = domainstation.NewPriceChanged(priced.ID, change, observedAt)
		}
		if err := h.outbox.StoreEvents(txCtx, events); err != nil {
			return messaging.Retryable(err)
		}
		priceChanges = len(changes)
		return nil
	})
	if err != nil {
		return err
	}
	if !skipped {
		log.Printf(
			"event handled %s eventId=%s stationId=%s priceChanges=%d",
			domainstation.EventTypePricesUpdated,
			msg.EventID,
			priced.StationID,
			priceChanges,
		)
	}
	return nil
}
