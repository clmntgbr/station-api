package station

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-api/internal/application/messaging"
	domainprice "go-api/internal/domain/price"
	domainstation "go-api/internal/domain/station"

	"github.com/google/uuid"
)

func TestUpsertedHandlerLeavesPricesOnInfoUpdate(t *testing.T) {
	built := mustFuelStation(t, 2334)
	msg := domainstation.NewStationUpserted(built, false)
	msg.EventID = "6ba7b814-9dad-11d1-80b4-00c04fd430c8"
	stations := &memoryStations{}
	handler := NewUpsertedHandler(stations, &memoryOutbox{}, &memoryProcessed{seen: map[string]bool{}})

	if err := handler.Handle(context.Background(), mustJSON(t, msg)); err != nil {
		t.Fatal(err)
	}
	if len(stations.synced) != 0 || len(stations.syncedInfo) != 1 || stations.syncedInfo[0] {
		t.Fatalf("synced info = %v prices = %d", stations.syncedInfo, len(stations.synced))
	}
}

func TestPricesUpdatedHandlerWritesPrices(t *testing.T) {
	built := mustFuelStation(t, 2334)
	msg := domainstation.NewPricesUpdated(built)
	msg.EventID = "6ba7b815-9dad-11d1-80b4-00c04fd430c8"
	stations := &memoryStations{
		present: map[uuid.UUID]bool{built.ID: true},
		prices: map[uuid.UUID][]domainprice.Price{
			built.ID: {{
				StationID:       built.StationID,
				Fuel:            domainprice.FuelGazole,
				Amount:          2300,
				SourceUpdatedAt: time.Date(2026, 10, 11, 8, 0, 0, 0, time.UTC),
			}},
		},
	}
	outbox := &memoryOutbox{}
	handler := NewPricesUpdatedHandler(stations, outbox, &memoryProcessed{seen: map[string]bool{}})

	if err := handler.Handle(context.Background(), mustJSON(t, msg)); err != nil {
		t.Fatal(err)
	}
	if len(stations.synced) != 1 || len(stations.synced[0].changes) != 1 || len(outbox.events) != 1 {
		t.Fatalf("synced = %+v events = %d", stations.synced, len(outbox.events))
	}

	if err := handler.Handle(context.Background(), mustJSON(t, msg)); err != nil {
		t.Fatal(err)
	}
	if len(stations.synced) != 1 {
		t.Fatal("replay wrote prices again")
	}
}

func TestPricesUpdatedHandlerRetriesBeforeCreation(t *testing.T) {
	built := mustFuelStation(t, 2334)
	msg := domainstation.NewPricesUpdated(built)
	msg.EventID = "6ba7b816-9dad-11d1-80b4-00c04fd430c8"
	handler := NewPricesUpdatedHandler(&memoryStations{}, &memoryOutbox{}, &memoryProcessed{seen: map[string]bool{}})

	err := handler.Handle(context.Background(), mustJSON(t, msg))
	var retryable *messaging.RetryableError
	if !errors.As(err, &retryable) {
		t.Fatalf("got %v", err)
	}
}
