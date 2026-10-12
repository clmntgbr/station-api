package station

import (
	"testing"
	"time"

	"go-api/internal/domain/price"
)

func TestDiffPrices(t *testing.T) {
	earlier := time.Date(2026, 9, 28, 11, 42, 0, 0, time.UTC)
	later := time.Date(2026, 10, 12, 10, 30, 46, 0, time.UTC)

	current := []price.Price{
		{StationID: "34150003", Fuel: price.FuelGazole, Amount: 2300, SourceUpdatedAt: earlier},
		{StationID: "34150003", Fuel: price.FuelSP98, Amount: 2500, SourceUpdatedAt: earlier},
	}
	incoming := []price.Price{
		{StationID: "34150003", Fuel: price.FuelGazole, Amount: 2334, SourceUpdatedAt: later},
		{StationID: "34150003", Fuel: price.FuelE10, Amount: 2200, SourceUpdatedAt: later},
	}

	changes := DiffPrices(current, incoming)
	if len(changes) != 2 {
		t.Fatalf("changes = %+v", changes)
	}
	if changes[0].Price.Fuel != price.FuelGazole || changes[0].PreviousAmount == nil || *changes[0].PreviousAmount != 2300 {
		t.Fatalf("gazole = %+v", changes[0])
	}
	if changes[1].Price.Fuel != price.FuelE10 || changes[1].PreviousAmount != nil {
		t.Fatalf("e10 = %+v", changes[1])
	}
}

func TestDiffPricesIgnoresSameAmount(t *testing.T) {
	earlier := time.Date(2026, 9, 28, 11, 42, 0, 0, time.UTC)
	later := time.Date(2026, 10, 12, 10, 30, 46, 0, time.UTC)
	current := []price.Price{{
		StationID: "34150003", Fuel: price.FuelGazole, Amount: 2334, SourceUpdatedAt: earlier,
	}}
	incoming := []price.Price{{
		StationID: "34150003", Fuel: price.FuelGazole, Amount: 2334, SourceUpdatedAt: later,
	}}

	if changes := DiffPrices(current, incoming); len(changes) != 0 {
		t.Fatalf("changes = %+v", changes)
	}
}
