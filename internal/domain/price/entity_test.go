package price

import (
	"errors"
	"testing"
	"time"
)

func TestNewRejectsInvalidPrice(t *testing.T) {
	updated := time.Date(2026, 10, 11, 8, 50, 9, 0, time.UTC)
	cases := []struct {
		name      string
		stationID string
		fuel      Fuel
		amount    int
	}{
		{name: "empty station", stationID: " ", fuel: FuelGazole, amount: 2399},
		{name: "unknown fuel", stationID: "89100001", fuel: "Diesel", amount: 2399},
		{name: "negative amount", stationID: "89100001", fuel: FuelGazole, amount: -1},
		{name: "overflow", stationID: "89100001", fuel: FuelGazole, amount: maxAmount + 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := New(tc.stationID, tc.fuel, tc.amount, updated)
			if !errors.Is(err, ErrInvalidPrice) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestCanonicalKeepsLatestPricePerFuel(t *testing.T) {
	earlier := time.Date(2026, 9, 28, 11, 42, 0, 0, time.UTC)
	later := time.Date(2026, 10, 11, 8, 50, 9, 0, time.UTC)

	got, err := Canonical([]Price{
		{StationID: "89100001", Fuel: FuelSP98, Amount: 2289, SourceUpdatedAt: earlier},
		{StationID: " 89100001 ", Fuel: FuelGazole, Amount: 1800, SourceUpdatedAt: earlier},
		{StationID: "89100001", Fuel: FuelGazole, Amount: 2399, SourceUpdatedAt: later},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Fuel != FuelGazole || got[0].Amount != 2399 || got[0].StationID != "89100001" {
		t.Fatalf("prices = %#v", got)
	}
	if got[1].Fuel != FuelSP98 || !got[1].SourceUpdatedAt.Equal(earlier) {
		t.Fatalf("sp98 = %#v", got[1])
	}
}

func TestCanonicalRejectsMixedStations(t *testing.T) {
	updated := time.Date(2026, 10, 11, 8, 50, 9, 0, time.UTC)
	_, err := Canonical([]Price{
		{StationID: "89100001", Fuel: FuelGazole, Amount: 2399, SourceUpdatedAt: updated},
		{StationID: "34150003", Fuel: FuelSP98, Amount: 2289, SourceUpdatedAt: updated},
	})
	if !errors.Is(err, ErrInvalidPrice) {
		t.Fatalf("got %v", err)
	}
}

func TestCanonicalEmpty(t *testing.T) {
	got, err := Canonical(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 || got == nil {
		t.Fatalf("prices = %#v", got)
	}
}
