package station

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"go-api/internal/application/messaging"
	"go-api/internal/domain/event"
	"go-api/internal/domain/port"
	domainprice "go-api/internal/domain/price"
	domainstation "go-api/internal/domain/station"

	"github.com/google/uuid"
)

func TestUpsertedHandlerWritesChangedPrices(t *testing.T) {
	observed := time.Date(2026, 10, 12, 14, 0, 0, 0, time.UTC)
	built := mustFuelStation(t, 2334)
	msg := domainstation.NewStationUpserted(built, true)
	msg.EventID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"

	previous := domainprice.Price{
		StationID:       built.StationID,
		Fuel:            domainprice.FuelGazole,
		Amount:          2300,
		SourceUpdatedAt: time.Date(2026, 10, 11, 8, 0, 0, 0, time.UTC),
	}
	stations := &memoryStations{prices: map[uuid.UUID][]domainprice.Price{built.ID: {previous}}}
	outbox := &memoryOutbox{}
	handler := NewUpsertedHandler(stations, outbox, &memoryProcessed{seen: map[string]bool{}})
	handler.now = func() time.Time { return observed }

	if err := handler.Handle(context.Background(), mustJSON(t, msg)); err != nil {
		t.Fatal(err)
	}
	if len(stations.saved) != 1 || stations.saved[0].ID != built.ID {
		t.Fatalf("saved = %+v", stations.saved)
	}
	if len(stations.synced) != 1 || len(stations.synced[0].changes) != 1 {
		t.Fatalf("synced = %+v", stations.synced)
	}
	if !stations.synced[0].observedAt.Equal(observed) {
		t.Fatalf("observed at %s", stations.synced[0].observedAt)
	}
	if len(outbox.events) != 1 {
		t.Fatalf("events = %d", len(outbox.events))
	}
	changed, ok := outbox.events[0].(domainstation.PriceChanged)
	if !ok || changed.Amount != 2334 || changed.PreviousAmount == nil || *changed.PreviousAmount != 2300 {
		t.Fatalf("event = %#v", outbox.events[0])
	}

	if err := handler.Handle(context.Background(), mustJSON(t, msg)); err != nil {
		t.Fatal(err)
	}
	if len(stations.saved) != 1 || len(outbox.events) != 1 {
		t.Fatal("replay wrote the station again")
	}
}

func TestUpsertedHandlerSkipsUnchangedAmount(t *testing.T) {
	built := mustFuelStation(t, 2334)
	msg := domainstation.NewStationUpserted(built, true)
	msg.EventID = "6ba7b811-9dad-11d1-80b4-00c04fd430c8"
	stations := &memoryStations{prices: map[uuid.UUID][]domainprice.Price{
		built.ID: {built.Prices[0]},
	}}
	outbox := &memoryOutbox{}
	handler := NewUpsertedHandler(stations, outbox, &memoryProcessed{seen: map[string]bool{}})

	if err := handler.Handle(context.Background(), mustJSON(t, msg)); err != nil {
		t.Fatal(err)
	}
	if len(stations.synced) != 1 || len(stations.synced[0].changes) != 0 || len(outbox.events) != 0 {
		t.Fatalf("synced = %+v events = %d", stations.synced, len(outbox.events))
	}
}

func TestUpsertedHandlerUsesEnvelopeEventID(t *testing.T) {
	built := mustFuelStation(t, 2334)
	msg := domainstation.NewStationUpserted(built, true)
	stations := &memoryStations{}
	handler := NewUpsertedHandler(stations, &memoryOutbox{}, &memoryProcessed{seen: map[string]bool{}})
	ctx := messaging.WithEventID(context.Background(), "6ba7b813-9dad-11d1-80b4-00c04fd430c8")

	if err := handler.Handle(ctx, mustJSON(t, msg)); err != nil {
		t.Fatal(err)
	}
	if len(stations.saved) != 1 {
		t.Fatalf("saved = %d", len(stations.saved))
	}
}

func TestUpsertedHandlerRejectsInvalidPayload(t *testing.T) {
	handler := NewUpsertedHandler(&memoryStations{}, &memoryOutbox{}, &memoryProcessed{seen: map[string]bool{}})

	err := handler.Handle(context.Background(), []byte("{"))
	var nonRetryable *messaging.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("got %v", err)
	}

	built := mustFuelStation(t, 2334)
	msg := domainstation.NewStationUpserted(built, true)
	msg.EventID = "6ba7b812-9dad-11d1-80b4-00c04fd430c8"
	msg.InfoHash = "nope"
	err = handler.Handle(context.Background(), mustJSON(t, msg))
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("got %v", err)
	}
}

func mustFuelStation(t *testing.T, amount int) domainstation.Station {
	t.Helper()
	s, err := domainstation.New(domainstation.Station{
		ID:         uuid.MustParse("0199a1b2-c3d4-7000-8000-000000000001"),
		StationID:  "34150003",
		EnergyType: domainstation.EnergyTypeFuel,
		Location:   domainstation.Location{Latitude: 43.651, Longitude: 3.547},
		Address:    "Route de Pézenas",
		City:       "GIGNAC",
		Pop:        domainstation.PopRoad,
		Prices: []domainprice.Price{{
			Fuel:            domainprice.FuelGazole,
			Amount:          amount,
			SourceUpdatedAt: time.Date(2026, 10, 12, 10, 30, 46, 0, time.UTC),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

type memoryStations struct {
	prices     map[uuid.UUID][]domainprice.Price
	present    map[uuid.UUID]bool
	saved      []domainstation.Station
	syncedInfo []bool
	synced     []syncedPrices
}

type syncedPrices struct {
	stationID  uuid.UUID
	prices     []domainprice.Price
	changes    []domainstation.PriceChange
	observedAt time.Time
}

func (m *memoryStations) WithTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func (m *memoryStations) Upsert(ctx context.Context, station domainstation.Station, syncPrices bool) (uuid.UUID, error) {
	m.saved = append(m.saved, station)
	m.syncedInfo = append(m.syncedInfo, syncPrices)
	return station.ID, nil
}

func (m *memoryStations) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	return m.present[id], nil
}

func (m *memoryStations) FindPrices(ctx context.Context, id uuid.UUID) ([]domainprice.Price, error) {
	return m.prices[id], nil
}

func (m *memoryStations) SyncPrices(ctx context.Context, id uuid.UUID, prices []domainprice.Price, changes []domainstation.PriceChange, observedAt time.Time, priceHash string) error {
	m.synced = append(m.synced, syncedPrices{
		stationID:  id,
		prices:     prices,
		changes:    changes,
		observedAt: observedAt,
	})
	return nil
}

type memoryProcessed struct {
	seen map[string]bool
}

func (m *memoryProcessed) MarkProcessed(ctx context.Context, eventID, handlerName string) (bool, error) {
	key := eventID + "/" + handlerName
	if m.seen[key] {
		return false, nil
	}
	m.seen[key] = true
	return true, nil
}

type memoryOutbox struct {
	events []event.DomainEvent
}

func (m *memoryOutbox) StoreEvents(ctx context.Context, events []event.DomainEvent) error {
	m.events = append(m.events, events...)
	return nil
}

func (m *memoryOutbox) FetchUnpublished(ctx context.Context, limit int) ([]port.OutboxMessage, error) {
	return nil, nil
}

func (m *memoryOutbox) MarkPublished(ctx context.Context, ids []uuid.UUID) error {
	return nil
}
