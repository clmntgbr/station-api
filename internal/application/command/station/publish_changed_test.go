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
)

func TestPublishChangedSkipsMatchingHash(t *testing.T) {
	sens := mustStation(t, "89100001", "SENS")
	gignac := mustStation(t, "34150003", "GIGNAC")
	publisher := &fakePublisher{}
	handler := NewPublishChangedHandler(
		staticFeed{sens, gignac},
		hashMap{sens.ID: sens.ContentHash()},
		publisher,
	)
	handler.now = func() time.Time {
		return time.Date(2026, 10, 12, 14, 0, 0, 0, time.UTC)
	}

	result, err := handler.Handle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Stations != 2 || result.Published != 1 || result.Unchanged != 1 {
		t.Fatalf("result = %+v", result)
	}
	if len(publisher.envelopes) != 1 {
		t.Fatalf("published %d events", len(publisher.envelopes))
	}

	got := publisher.envelopes[0]
	if got.Type != domainstation.EventTypeStationUpserted || got.AggregateID != gignac.ID {
		t.Fatalf("envelope = %+v", got)
	}
	if got.EventID == "" || got.OccurredAt != "2026-10-12T14:00:00Z" {
		t.Fatalf("identity = %s %s", got.EventID, got.OccurredAt)
	}

	var payload domainstation.StationUpserted
	if err := json.Unmarshal(got.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ContentHash != gignac.ContentHash() || payload.City != "GIGNAC" {
		t.Fatalf("payload = %+v", payload)
	}

	again, err := handler.Handle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if again.Published != 1 || publisher.envelopes[1].EventID != got.EventID {
		t.Fatal("republishing the same hash changed the event id")
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

type hashMap map[string]string

func (m hashMap) FindContentHashes(ctx context.Context) (map[string]string, error) {
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
		ID:         id,
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
