package station

import (
	"errors"
	"testing"
	"time"

	"go-api/internal/domain/price"

	"github.com/google/uuid"
)

func TestNewRejectsInvalidStation(t *testing.T) {
	updated := time.Date(2026, 10, 11, 8, 50, 9, 0, time.UTC)
	base := Station{
		StationID:  "89100001",
		EnergyType: EnergyTypeFuel,
		Location:   Location{Latitude: 48.183, Longitude: 3.309},
		PostalCode: "89100",
		Address:    "84 ROUTE DE MAILLOT",
		City:       "SENS",
		Pop:        PopRoad,
		Hours: Hours{
			Days: []Day{{
				ID:   1,
				Name: "Lundi",
				Slots: []Slot{{
					Opens:  Clock{Hour: 8, Minute: 0},
					Closes: Clock{Hour: 19, Minute: 0},
				}},
			}},
		},
		Services: []string{"Toilettes publiques"},
		Prices: []price.Price{{
			Fuel:            price.FuelGazole,
			Amount:          2399,
			SourceUpdatedAt: updated,
		}},
	}

	cases := []struct {
		name string
		edit func(*Station)
		want error
	}{
		{name: "empty id", edit: func(s *Station) { s.StationID = " " }, want: ErrInvalidStation},
		{name: "uuid version", edit: func(s *Station) {
			s.ID = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
		}, want: ErrInvalidStation},
		{name: "bad energy", edit: func(s *Station) { s.EnergyType = "gas" }, want: ErrInvalidStation},
		{name: "null island", edit: func(s *Station) { s.Location = Location{} }, want: ErrInvalidLocation},
		{name: "latitude out of range", edit: func(s *Station) { s.Location.Latitude = 91 }, want: ErrInvalidLocation},
		{name: "longitude out of range", edit: func(s *Station) { s.Location.Longitude = 181 }, want: ErrInvalidLocation},
		{name: "unknown fuel", edit: func(s *Station) { s.Prices[0].Fuel = "Diesel" }, want: price.ErrInvalidPrice},
		{name: "negative price", edit: func(s *Station) { s.Prices[0].Amount = -1 }, want: price.ErrInvalidPrice},
		{name: "price overflow", edit: func(s *Station) { s.Prices[0].Amount = 1_000_000 }, want: price.ErrInvalidPrice},
		{name: "foreign price", edit: func(s *Station) { s.Prices[0].StationID = "other" }, want: price.ErrInvalidPrice},
		{name: "bad clock", edit: func(s *Station) { s.Hours.Days[0].Slots[0].Opens.Hour = 24 }, want: ErrInvalidStation},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := base
			input.Hours.Days = append([]Day(nil), base.Hours.Days...)
			input.Hours.Days[0].Slots = append([]Slot(nil), base.Hours.Days[0].Slots...)
			input.Prices = append([]price.Price(nil), base.Prices...)
			tc.edit(&input)

			_, err := New(input)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestNewCanonicalizesStation(t *testing.T) {
	earlier := time.Date(2026, 9, 28, 11, 42, 0, 0, time.UTC)
	later := time.Date(2026, 10, 11, 8, 50, 9, 0, time.UTC)

	got, err := New(Station{
		StationID:  " 89100001 ",
		EnergyType: EnergyTypeFuel,
		Location:   Location{Latitude: 48.183, Longitude: 3.309},
		City:       " SENS ",
		Pop:        PopRoad,
		Hours: Hours{
			Automate24: true,
			Days: []Day{
				{ID: 2, Name: " Mardi ", Closed: false},
				{
					ID:     1,
					Name:   "Lundi",
					Closed: true,
					Slots: []Slot{{
						Opens:  Clock{Hour: 1, Minute: 0},
						Closes: Clock{Hour: 1, Minute: 0},
					}},
				},
			},
		},
		Services: []string{" Toilettes publiques ", "", "Wifi"},
		Prices: []price.Price{
			{Fuel: price.FuelSP98, Amount: 2289, SourceUpdatedAt: earlier},
			{Fuel: price.FuelGazole, Amount: 1800, SourceUpdatedAt: earlier},
			{Fuel: price.FuelGazole, Amount: 2399, SourceUpdatedAt: later},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if got.StationID != "89100001" || got.City != "SENS" {
		t.Fatalf("identity not trimmed: %#v", got)
	}
	if got.Hours.Days[0].ID != 1 || got.Hours.Days[0].Name != "Lundi" || !got.Hours.Days[0].Closed {
		t.Fatalf("days not ordered: %#v", got.Hours.Days)
	}
	if got.Hours.Days[0].Slots[0].Opens.Hour != 1 || got.Hours.Days[1].Name != "Mardi" {
		t.Fatalf("hours not preserved: %#v", got.Hours.Days)
	}
	if len(got.Services) != 2 || got.Services[0] != "Toilettes publiques" || got.Services[1] != "Wifi" {
		t.Fatalf("services = %#v", got.Services)
	}
	if len(got.Prices) != 2 || got.Prices[0].Fuel != price.FuelGazole || got.Prices[0].Amount != 2399 || got.Prices[0].StationID != "89100001" || got.Prices[1].Fuel != price.FuelSP98 {
		t.Fatalf("prices = %#v", got.Prices)
	}
	if got.Services == nil || got.Prices == nil || got.Hours.Days == nil {
		t.Fatal("canonical slices must be empty, not nil")
	}
}

func TestNewAcceptsStationWithoutPrices(t *testing.T) {
	got, err := New(Station{
		StationID:  "58240003",
		EnergyType: EnergyTypeFuel,
		Location:   Location{Latitude: 46.7535171497, Longitude: 3.14069121289},
		City:       "Chantenay-Saint-Imbert",
		Pop:        PopRoad,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Prices) != 0 || got.Prices == nil {
		t.Fatalf("prices = %#v", got.Prices)
	}
	if len(got.Services) != 0 || got.Services == nil {
		t.Fatalf("services = %#v", got.Services)
	}
	if len(got.Hours.Days) != 0 || got.Hours.Days == nil {
		t.Fatalf("days = %#v", got.Hours.Days)
	}
}

func TestHashesSplitInfoAndPrices(t *testing.T) {
	first, err := New(sampleStation())
	if err != nil {
		t.Fatal(err)
	}
	second, err := New(sampleStation())
	if err != nil {
		t.Fatal(err)
	}
	if first.InfoHash() == "" || first.PriceHash() == "" || first.InfoHash() == first.PriceHash() {
		t.Fatalf("hashes = %s %s", first.InfoHash(), first.PriceHash())
	}
	if first.InfoHash() != second.InfoHash() || first.PriceHash() != second.PriceHash() {
		t.Fatal("hashes are not stable")
	}

	priced := sampleStation()
	priced.Prices[0].Amount = 2400
	priceChanged, err := New(priced)
	if err != nil {
		t.Fatal(err)
	}
	if priceChanged.InfoHash() != first.InfoHash() || priceChanged.PriceHash() == first.PriceHash() {
		t.Fatal("a price change must affect only the price hash")
	}

	served := sampleStation()
	served.Services = []string{"Toilettes publiques"}
	infoChanged, err := New(served)
	if err != nil {
		t.Fatal(err)
	}
	if infoChanged.PriceHash() != first.PriceHash() || infoChanged.InfoHash() == first.InfoHash() {
		t.Fatal("a service change must affect only the info hash")
	}
}

func sampleStation() Station {
	return Station{
		StationID:  "34150003",
		EnergyType: EnergyTypeFuel,
		Location:   Location{Latitude: 43.651, Longitude: 3.547},
		PostalCode: "34150",
		Address:    "Route de Pézenas",
		City:       "GIGNAC",
		Pop:        PopRoad,
		Hours:      Hours{Automate24: true},
		Services:   []string{"Station de gonflage", "Lavage automatique"},
		Prices: []price.Price{{
			Fuel:            price.FuelGazole,
			Amount:          2334,
			SourceUpdatedAt: time.Date(2026, 10, 12, 8, 30, 46, 0, time.UTC),
		}},
	}
}
