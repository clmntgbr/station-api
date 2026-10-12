package station

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go-api/internal/domain/event"
	"go-api/internal/domain/port"
	domainstation "go-api/internal/domain/station"
)

// QueueName is the worker queue bound to station.upserted.v1.
const QueueName = "station.upserted"

// Feed streams stations from the open-data archive.
type Feed interface {
	Stream(ctx context.Context, yield func(domainstation.Station) error) error
}

// Result counts one pass over the feed.
type Result struct {
	Stations  int
	Published int
	Unchanged int
}

// PublishChangedHandler publishes station.upserted.v1 when the content hash
// differs from the stored one. An equal hash publishes nothing.
type PublishChangedHandler struct {
	feed      Feed
	hashes    domainstation.ContentHashReader
	publisher port.EventPublisher
	now       func() time.Time
	progress  func(read int)
}

func NewPublishChangedHandler(feed Feed, hashes domainstation.ContentHashReader, publisher port.EventPublisher) *PublishChangedHandler {
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
	known, err := h.hashes.FindContentHashes(ctx)
	if err != nil {
		return Result{}, err
	}
	if known == nil {
		known = map[string]string{}
	}

	var result Result
	err = h.feed.Stream(ctx, func(s domainstation.Station) error {
		result.Stations++
		if h.progress != nil && result.Stations%1000 == 0 {
			h.progress(result.Stations)
		}

		hash := s.ContentHash()
		if known[s.ID] == hash {
			result.Unchanged++
			return nil
		}

		payload, err := json.Marshal(domainstation.NewStationUpserted(s))
		if err != nil {
			return fmt.Errorf("encode station %s: %w", s.ID, err)
		}
		occurredAt := h.now().UTC()
		err = h.publisher.Publish(ctx, port.EventEnvelope{
			EventID:     event.DeterministicID(domainstation.EventTypeStationUpserted, s.ID, hash),
			Type:        domainstation.EventTypeStationUpserted,
			AggregateID: s.ID,
			OccurredAt:  occurredAt.Format(time.RFC3339Nano),
			Payload:     payload,
		})
		if err != nil {
			return fmt.Errorf("publish station %s: %w", s.ID, err)
		}
		result.Published++
		return nil
	})
	if err != nil {
		return result, err
	}
	return result, nil
}
