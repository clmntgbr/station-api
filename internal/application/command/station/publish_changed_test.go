package station

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"go-api/internal/domain/port"
	domainprice "go-api/internal/domain/price"
	domainstation "go-api/internal/domain/station"

	"github.com/google/uuid"
)

func TestPublishChangedSkipsMatchingHash(t *testing.T) {
	sens := mustStation(t, "89100001", "SENS")
	gignac := mustStation(t, "34150003", "GIGNAC")
	publisher := &fakePublisher{}
	handler := NewPublishChangedHandler(
		staticFeed{sens, gignac},
		hashMap{sens.StationID: domainstation.Hashes{Info: sens.InfoHash(), Price: sens.PriceHash()}},
		publisher,
	)
	handler.now = func() time.Time {
		return time.Date(2026, 10, 12, 14, 0, 0, 0, time.UTC)
	}

	result, err := handler.Handle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Stations != 2 || result.Created != 1 || result.Info != 0 || result.Prices != 0 || result.Unchanged != 1 {
		t.Fatalf("result = %+v", result)
	}
	if len(publisher.envelopes) != 1 {
		t.Fatalf("published %d events", len(publisher.envelopes))
	}

	got := publisher.envelopes[0]
	var payload domainstation.StationUpserted
	if err := json.Unmarshal(got.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if got.Type != domainstation.EventTypeStationUpserted || payload.StationID != gignac.StationID || payload.ID.Version() != 7 || got.AggregateID != payload.ID.String() {
		t.Fatalf("envelope = %+v", got)
	}
	if got.EventID == "" || got.OccurredAt != "2026-10-12T14:00:00Z" {
		t.Fatalf("identity = %s %s", got.EventID, got.OccurredAt)
	}

	if payload.EventID != got.EventID || !payload.SyncPrices || payload.InfoHash != gignac.InfoHash() || payload.PriceHash != gignac.PriceHash() || payload.City != "GIGNAC" {
		t.Fatalf("payload = %+v", payload)
	}

	again, err := handler.Handle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if again.Created != 1 || publisher.envelopes[1].EventID != got.EventID {
		t.Fatal("republishing the same station changed the event id")
	}
}

func TestPublishChangedSplitsTasks(t *testing.T) {
	current := mustStation(t, "34150003", "GIGNAC")
	infoChanged := current
	infoChanged.City = "PEZENAS"
	priceChanged := current
	priceChanged.Prices = []domainprice.Price{current.Prices[0]}
	priceChanged.Prices[0].Amount = 2400
	both := priceChanged
	both.City = "PEZENAS"

	knownID := uuid.MustParse("0199a1b2-c3d4-7000-8000-000000000002")
	known := hashMap{current.StationID: {ID: knownID, Info: current.InfoHash(), Price: current.PriceHash()}}
	publisher := &fakePublisher{}
	handler := NewPublishChangedHandler(staticFeed{infoChanged, priceChanged, both}, known, publisher)

	result, err := handler.Handle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Created != 0 || result.Info != 2 || result.Prices != 2 || result.Unchanged != 0 {
		t.Fatalf("result = %+v", result)
	}
	if len(publisher.envelopes) != 4 {
		t.Fatalf("published %d events", len(publisher.envelopes))
	}
	if publisher.envelopes[0].Type != domainstation.EventTypeStationUpserted || publisher.envelopes[1].Type != domainstation.EventTypePricesUpdated {
		t.Fatalf("order = %s %s", publisher.envelopes[0].Type, publisher.envelopes[1].Type)
	}

	var info domainstation.StationUpserted
	if err := json.Unmarshal(publisher.envelopes[0].Payload, &info); err != nil {
		t.Fatal(err)
	}
	if info.SyncPrices || len(info.Prices) != 0 || info.City != "PEZENAS" || info.ID != knownID || info.StationID != current.StationID {
		t.Fatalf("info task = %+v", info)
	}
	var prices domainstation.PricesUpdated
	if err := json.Unmarshal(publisher.envelopes[1].Payload, &prices); err != nil {
		t.Fatal(err)
	}
	if prices.ID != knownID || prices.StationID != current.StationID || prices.PriceHash != priceChanged.PriceHash() || len(prices.Prices) != 1 || prices.Prices[0].Amount != 2400 {
		t.Fatalf("price task = %+v", prices)
	}
}

func TestPublishChangedStopsOnPublishError(t *testing.T) {
	sens := mustStation(t, "89100001", "SENS")
	handler := NewPublishChangedHandler(staticFeed{sens}, hashMap{}, &fakePublisher{err: errors.New("broker down")})

	_, err := handler.Handle(context.Background())
	if err == nil {
		t.Fatal("expected publish error")
	}
}

type staticFeed []domainstation.Station

func (f staticFeed) Stream(ctx context.Context, yield func(domainstation.Station) error) error {
	for _, s := range f {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := yield(s); err != nil {
			return err
		}
	}
	return nil
}

type hashMap map[string]domainstation.Hashes

func (m hashMap) FindHashes(ctx context.Context) (map[string]domainstation.Hashes, error) {
	if m == nil {
		return map[string]domainstation.Hashes{}, nil
	}
	return m, nil
}

type fakePublisher struct {
	envelopes []port.EventEnvelope
	err       error
}

func (f *fakePublisher) Publish(ctx context.Context, envelope port.EventEnvelope) error {
	if f.err != nil {
		return f.err
	}
	f.envelopes = append(f.envelopes, envelope)
	return nil
}

func mustStation(t *testing.T, id, city string) domainstation.Station {
	t.Helper()
	s, err := domainstation.New(domainstation.Station{
		StationID:  id,
		EnergyType: domainstation.EnergyTypeFuel,
		Location:   domainstation.Location{Latitude: 48.183, Longitude: 3.309},
		City:       city,
		Pop:        domainstation.PopRoad,
		Prices: []domainprice.Price{{
			Fuel:            domainprice.FuelGazole,
			Amount:          2399,
			SourceUpdatedAt: time.Date(2026, 10, 11, 8, 50, 9, 0, time.UTC),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}
