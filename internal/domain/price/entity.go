package price

import (
	"errors"
	"sort"
	"strings"
	"time"
)

const maxAmount = 999999

// Fuel is a pump product.
type Fuel string

const (
	FuelGazole Fuel = "Gazole"
	FuelSP95   Fuel = "SP95"
	FuelE85    Fuel = "E85"
	FuelGPLc   Fuel = "GPLc"
	FuelE10    Fuel = "E10"
	FuelSP98   Fuel = "SP98"
)

// Valid reports whether the fuel is one of the products published by the feed.
func (f Fuel) Valid() bool {
	switch f {
	case FuelGazole, FuelSP95, FuelE85, FuelGPLc, FuelE10, FuelSP98:
		return true
	default:
		return false
	}
}

func (f Fuel) rank() int {
	switch f {
	case FuelGazole:
		return 1
	case FuelSP95:
		return 2
	case FuelE85:
		return 3
	case FuelGPLc:
		return 4
	case FuelE10:
		return 5
	case FuelSP98:
		return 6
	default:
		return 0
	}
}

// ErrInvalidPrice means the station id, fuel, or amount cannot be stored.
var ErrInvalidPrice = errors.New("invalid price")

// Price is the latest pump price of one fuel at one station.
// Identity is the pair (StationID, Fuel).
// StationID is the feed identifier, not the station's internal UUID.
// Amount is thousandths of a euro per litre: 2399 means 2.399 €/L.
type Price struct {
	StationID       string
	Fuel            Fuel
	Amount          int
	SourceUpdatedAt time.Time
}

// New validates one pump price.
func New(stationID string, fuel Fuel, amount int, sourceUpdatedAt time.Time) (Price, error) {
	stationID = strings.TrimSpace(stationID)
	if stationID == "" || !fuel.Valid() || amount < 0 || amount > maxAmount {
		return Price{}, ErrInvalidPrice
	}
	return Price{
		StationID:       stationID,
		Fuel:            fuel,
		Amount:          amount,
		SourceUpdatedAt: sourceUpdatedAt.UTC(),
	}, nil
}

// Canonical validates prices, keeps the latest amount per fuel, and orders them by fuel.
// Every price must belong to the same station.
func Canonical(prices []Price) ([]Price, error) {
	if len(prices) == 0 {
		return []Price{}, nil
	}

	kept := make(map[Fuel]Price, len(prices))
	var stationID string
	for _, candidate := range prices {
		built, err := New(candidate.StationID, candidate.Fuel, candidate.Amount, candidate.SourceUpdatedAt)
		if err != nil {
			return nil, err
		}
		if stationID == "" {
			stationID = built.StationID
		} else if built.StationID != stationID {
			return nil, ErrInvalidPrice
		}
		previous, ok := kept[built.Fuel]
		if !ok || !built.SourceUpdatedAt.Before(previous.SourceUpdatedAt) {
			kept[built.Fuel] = built
		}
	}

	out := make([]Price, 0, len(kept))
	for _, built := range kept {
		out = append(out, built)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Fuel.rank() < out[j].Fuel.rank()
	})
	return out, nil
}
