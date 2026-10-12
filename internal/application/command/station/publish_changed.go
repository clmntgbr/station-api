package station

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go-api/internal/domain/event"
	"go-api/internal/domain/port"
	domainstation "go-api/internal/domain/station"

	"github.com/google/uuid"
)

// QueueName is the worker queue bound to the station tasks.
const QueueName = "station.upserted"

// RoutingKeys binds the station task and the price task to QueueName.
func RoutingKeys() string {
	return domainstation.EventTypeStationUpserted + "," + domainstation.EventTypePricesUpdated
}

// Feed streams stations from the open-data archive.
type Feed interface {
	Stream(ctx context.Context, yield func(domainstation.Station) error) error
}

// Result counts one pass over the feed.
type Result struct {
	Stations  int
	Created   int
	Info      int
	Prices    int
	Unchanged int
}

// PublishChangedHandler publishes one task per hash that differs.
// A station that is not in the database yet is created by a single task that
// also writes its prices, so the price task never runs before the row exists.
type PublishChangedHandler struct {
	feed      Feed
	hashes    domainstation.HashReader
	publisher port.EventPublisher
	now       func() time.Time
	progress  func(read int)
}

func NewPublishChangedHandler(feed Feed, hashes domainstation.HashReader, publisher port.EventPublisher) *PublishChangedHandler {
	return &PublishChangedHandler{
		feed:      feed,
		hashes:    hashes,
		publisher: publisher,
		now:       time.Now,
	}
}

// OnProgress is called every 1000 stations read from the feed.
func (h *PublishChangedHandler) OnProgress(fn func(read int)) {
	h.progress = fn
}

func (h *PublishChangedHandler) Handle(ctx context.Context) (Result, error) {
	known, err := h.hashes.FindHashes(ctx)
	if err != nil {
		return Result{}, err
	}
	if known == nil {
		known = map[string]domainstation.Hashes{}
	}

	var result Result
	err = h.feed.Stream(ctx, func(s domainstation.Station) error {
		result.Stations++
		if h.progress != nil && result.Stations%1000 == 0 {
			h.progress(result.Stations)
		}

		stored, exists := known[s.StationID]
		infoHash := s.InfoHash()
		priceHash := s.PriceHash()
		occurredAt := h.now().UTC()
		if !exists {
			id, err := uuid.NewV7()
			if err != nil {
				return fmt.Errorf("station id for %s: %w", s.StationID, err)
			}
			s.ID = id
			if err := h.publishStation(ctx, s, true, infoHash, occurredAt); err != nil {
				return err
			}
			result.Created++
			return nil
		}
		s.ID = stored.ID
		if stored.Info == infoHash && stored.Price == priceHash {
			result.Unchanged++
			return nil
		}
		if stored.Info != infoHash {
			if err := h.publishStation(ctx, s, false, infoHash, occurredAt); err != nil {
				return err
			}
			result.Info++
		}
		if stored.Price != priceHash {
			if err := h.publishPrices(ctx, s, priceHash, occurredAt); err != nil {
				return err
			}
			result.Prices++
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	return result, nil
}

func (h *PublishChangedHandler) publishStation(ctx context.Context, s domainstation.Station, syncPrices bool, infoHash string, occurredAt time.Time) error {
	upserted := domainstation.NewStationUpserted(s, syncPrices)
	upserted.EventID = event.DeterministicID(domainstation.EventTypeStationUpserted, s.StationID, infoHash)
	return h.publish(ctx, upserted.EventID, domainstation.EventTypeStationUpserted, s.ID, occurredAt, upserted)
}

func (h *PublishChangedHandler) publishPrices(ctx context.Context, s domainstation.Station, priceHash string, occurredAt time.Time) error {
	updated := domainstation.NewPricesUpdated(s)
	updated.EventID = event.DeterministicID(domainstation.EventTypePricesUpdated, s.StationID, priceHash)
	return h.publish(ctx, updated.EventID, domainstation.EventTypePricesUpdated, s.ID, occurredAt, updated)
}

func (h *PublishChangedHandler) publish(ctx context.Context, eventID, eventType string, id uuid.UUID, occurredAt time.Time, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode station %s: %w", id, err)
	}
	err = h.publisher.Publish(ctx, port.EventEnvelope{
		EventID:     eventID,
		Type:        eventType,
		AggregateID: id.String(),
		OccurredAt:  occurredAt.Format(time.RFC3339Nano),
		Payload:     body,
	})
	if err != nil {
		return fmt.Errorf("publish station %s: %w", id, err)
	}
	return nil
}
