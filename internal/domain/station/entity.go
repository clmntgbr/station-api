package station

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"go-api/internal/domain/price"
)

const (
	// EnergyTypeFuel is a fuel station.
	EnergyTypeFuel EnergyType = "fuel"
	// EnergyTypeEV is an electric charging station.
	EnergyTypeEV EnergyType = "ev"

	// PopRoad is a roadside station (feed value "R").
	PopRoad = "R"
	// PopMotorway is a motorway station (feed value "A").
	PopMotorway = "A"
)

// EnergyType distinguishes fuel stations from electric charging stations.
type EnergyType string

// Valid reports whether the energy type is known.
func (e EnergyType) Valid() bool {
	return e == EnergyTypeFuel || e == EnergyTypeEV
}

var (
	// ErrInvalidStation means the station identity or schedule cannot be stored.
	ErrInvalidStation = errors.New("invalid station")
	// ErrInvalidLocation means the station has no usable WGS84 position.
	ErrInvalidLocation = errors.New("invalid station location")
)

// Station is a fuel or charging site built from an open-data feed.
type Station struct {
	ID         string
	EnergyType EnergyType
	Location   Location
	PostalCode string
	Address    string
	City       string
	// Pop is the roadside category from the feed: PopRoad or PopMotorway.
	Pop      string
	Hours    Hours
	Services []string
	Prices   []price.Price
}

// New validates a station and returns a canonical copy.
// Services keep their feed order. Days are ordered by weekday and prices by fuel,
// so ContentHash does not change when the feed reorders those lists.
// A station without prices is valid.
func New(s Station) (Station, error) {
	s.ID = strings.TrimSpace(s.ID)
	s.PostalCode = strings.TrimSpace(s.PostalCode)
	s.Address = strings.TrimSpace(s.Address)
	s.City = strings.TrimSpace(s.City)
	s.Pop = strings.TrimSpace(s.Pop)

	if s.ID == "" || !s.EnergyType.Valid() {
		return Station{}, ErrInvalidStation
	}
	if !s.Location.Valid() {
		return Station{}, ErrInvalidLocation
	}

	days, err := canonicalDays(s.Hours.Days)
	if err != nil {
		return Station{}, err
	}
	prices, err := canonicalPrices(s.ID, s.Prices)
	if err != nil {
		return Station{}, err
	}

	s.Hours.Days = days
	s.Services = cleanServices(s.Services)
	s.Prices = prices
	return s, nil
}

// ContentHash is the fingerprint of the feed content.
// The ingester publishes a station only when this value differs from the stored one.
// The hash is versioned with a leading "v1" line.
func (s Station) ContentHash() string {
	var b strings.Builder
	line := func(format string, args ...any) {
		fmt.Fprintf(&b, format, args...)
		b.WriteByte('\n')
	}

	line("v1")
	line("id=%s", s.ID)
	line("energy=%s", s.EnergyType)
	line("lat=%s", strconv.FormatFloat(s.Location.Latitude, 'f', -1, 64))
	line("lng=%s", strconv.FormatFloat(s.Location.Longitude, 'f', -1, 64))
	line("cp=%s", s.PostalCode)
	line("address=%s", s.Address)
	line("city=%s", s.City)
	line("pop=%s", s.Pop)
	if s.Hours.Automate24 {
		line("automate24=1")
	} else {
		line("automate24=0")
	}
	for _, day := range s.Hours.Days {
		closed := "0"
		if day.Closed {
			closed = "1"
		}
		slots := make([]string, len(day.Slots))
		for i, slot := range day.Slots {
			slots[i] = fmt.Sprintf("%02d:%02d-%02d:%02d", slot.Opens.Hour, slot.Opens.Minute, slot.Closes.Hour, slot.Closes.Minute)
		}
		line("day=%d|%s|%s|%s", day.ID, day.Name, closed, strings.Join(slots, ","))
	}
	for _, service := range s.Services {
		line("service=%s", service)
	}
	for _, price := range s.Prices {
		line("price=%s|%d|%s", price.Fuel, price.Amount, price.SourceUpdatedAt.UTC().Format(time.RFC3339))
	}

	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func canonicalDays(days []Day) ([]Day, error) {
	if len(days) == 0 {
		return []Day{}, nil
	}
	out := make([]Day, len(days))
	for i, day := range days {
		day.Name = strings.TrimSpace(day.Name)
		if len(day.Slots) == 0 {
			day.Slots = []Slot{}
		} else {
			day.Slots = append([]Slot(nil), day.Slots...)
		}
		for _, slot := range day.Slots {
			if !slot.Opens.Valid() || !slot.Closes.Valid() {
				return nil, ErrInvalidStation
			}
		}
		out[i] = day
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func canonicalPrices(stationID string, prices []price.Price) ([]price.Price, error) {
	if len(prices) == 0 {
		return []price.Price{}, nil
	}
	owned := make([]price.Price, len(prices))
	for i, item := range prices {
		itemStationID := strings.TrimSpace(item.StationID)
		if itemStationID == "" {
			item.StationID = stationID
		} else if itemStationID != stationID {
			return nil, price.ErrInvalidPrice
		}
		owned[i] = item
	}
	return price.Canonical(owned)
}

func cleanServices(services []string) []string {
	out := make([]string, 0, len(services))
	for _, service := range services {
		service = strings.TrimSpace(service)
		if service == "" {
			continue
		}
		out = append(out, service)
	}
	return out
}
