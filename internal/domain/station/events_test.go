package station

import (
	"errors"
	"testing"
	"time"

	"go-api/internal/domain/price"

	"github.com/google/uuid"
)

var testStationUUID = uuid.MustParse("0199a1b2-c3d4-7000-8000-000000000001")

func TestStationUpsertedRoundTrip(t *testing.T) {
	updated := time.Date(2026, 10, 12, 10, 30, 46, 0, time.UTC)
	original, err := New(Station{
		ID:         testStationUUID,
		StationID:  "34150003",
		EnergyType: EnergyTypeFuel,
		Location:   Location{Latitude: 43.651, Longitude: 3.547},
		PostalCode: "34150",
		Address:    "Route de Pézenas",
		City:       "GIGNAC",
		Pop:        PopRoad,
		Hours: Hours{
			Automate24: true,
			Days: []Day{{
				ID:     1,
				Name:   "Lundi",
				Closed: true,
				Slots: []Slot{{
					Opens:  Clock{Hour: 1, Minute: 0},
					Closes: Clock{Hour: 1, Minute: 0},
				}},
			}},
		},
		Services: []string{"Station de gonflage"},
		Prices: []price.Price{{
			Fuel:            price.FuelGazole,
			Amount:          2334,
			SourceUpdatedAt: updated,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	msg := NewStationUpserted(original, true)
	got, err := msg.Station()
	if err != nil {
		t.Fatal(err)
	}
	if got.InfoHash() != original.InfoHash() || got.PriceHash() != original.PriceHash() {
		t.Fatalf("hash %s/%s != %s/%s", got.InfoHash(), got.PriceHash(), original.InfoHash(), original.PriceHash())
	}
	if !got.Hours.Automate24 || got.Hours.Days[0].Slots[0].Opens.Hour != 1 || got.Address != "Route de Pézenas" {
		t.Fatalf("station = %+v", got)
	}

	msg.Days[0].Slots[0].Open = "25:00"
	if _, err := msg.Station(); !errors.Is(err, ErrInvalidStation) {
		t.Fatalf("got %v", err)
	}
}

func TestNewPriceChangedIsStable(t *testing.T) {
	updated := time.Date(2026, 10, 12, 10, 30, 46, 0, time.UTC)
	occurred := time.Date(2026, 10, 12, 14, 0, 0, 0, time.UTC)
	previous := 2300
	change := PriceChange{
		Price: price.Price{
			StationID:       "34150003",
			Fuel:            price.FuelGazole,
			Amount:          2334,
			SourceUpdatedAt: updated,
		},
		PreviousAmount: &previous,
	}

	first := NewPriceChanged(testStationUUID, change, occurred)
	second := NewPriceChanged(testStationUUID, change, occurred)
	if first.ID == "" || first.ID != second.ID {
		t.Fatalf("id %s / %s", first.ID, second.ID)
	}
	if first.EventType() != EventTypePriceChanged || first.AggregateID() != testStationUUID.String() || first.StationID != "34150003" || !first.OccurredAt().Equal(occurred) {
		t.Fatalf("event = %+v", first)
	}
	if first.PreviousAmount == nil || *first.PreviousAmount != 2300 || first.Amount != 2334 {
		t.Fatalf("amounts = %+v", first)
	}
}
