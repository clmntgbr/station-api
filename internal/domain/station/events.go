package station

import (
	"fmt"
	"time"

	"go-api/internal/domain/price"
)

const EventTypeStationUpserted = "station.upserted.v1"

// StationUpserted is the payload of station.upserted.v1.
// ContentHash is the fingerprint the ingester compared with the stored hash.
// Amount on each price is thousandths of a euro per litre.
type StationUpserted struct {
	ID          string               `json:"id"`
	EnergyType  EnergyType           `json:"energyType"`
	Latitude    float64              `json:"latitude"`
	Longitude   float64              `json:"longitude"`
	PostalCode  string               `json:"postalCode"`
	Address     string               `json:"address"`
	City        string               `json:"city"`
	Pop         string               `json:"pop"`
	Automate24  bool                 `json:"automate24"`
	Days        []StationUpsertDay   `json:"days"`
	Services    []string             `json:"services"`
	Prices      []StationUpsertPrice `json:"prices"`
	ContentHash string               `json:"contentHash"`
}

type StationUpsertDay struct {
	ID     int                 `json:"id"`
	Name   string              `json:"name"`
	Closed bool                `json:"closed"`
	Slots  []StationUpsertSlot `json:"slots"`
}

type StationUpsertSlot struct {
	Open  string `json:"open"`
	Close string `json:"close"`
}

// StationUpsertPrice is the published form of a price entity.
type StationUpsertPrice struct {
	StationID       string     `json:"stationId"`
	Fuel            price.Fuel `json:"fuel"`
	Amount          int        `json:"amount"`
	SourceUpdatedAt time.Time  `json:"sourceUpdatedAt"`
}

// NewStationUpserted builds the message payload from a station.
func NewStationUpserted(s Station) StationUpserted {
	days := make([]StationUpsertDay, len(s.Hours.Days))
	for i, day := range s.Hours.Days {
		slots := make([]StationUpsertSlot, len(day.Slots))
		for j, slot := range day.Slots {
			slots[j] = StationUpsertSlot{
				Open:  formatClock(slot.Opens),
				Close: formatClock(slot.Closes),
			}
		}
		days[i] = StationUpsertDay{
			ID:     day.ID,
			Name:   day.Name,
			Closed: day.Closed,
			Slots:  slots,
		}
	}

	services := s.Services
	if services == nil {
		services = []string{}
	}
	prices := make([]StationUpsertPrice, len(s.Prices))
	for i, item := range s.Prices {
		prices[i] = StationUpsertPrice{
			StationID:       item.StationID,
			Fuel:            item.Fuel,
			Amount:          item.Amount,
			SourceUpdatedAt: item.SourceUpdatedAt.UTC(),
		}
	}

	return StationUpserted{
		ID:          s.ID,
		EnergyType:  s.EnergyType,
		Latitude:    s.Location.Latitude,
		Longitude:   s.Location.Longitude,
		PostalCode:  s.PostalCode,
		Address:     s.Address,
		City:        s.City,
		Pop:         s.Pop,
		Automate24:  s.Hours.Automate24,
		Days:        days,
		Services:    services,
		Prices:      prices,
		ContentHash: s.ContentHash(),
	}
}

func formatClock(clock Clock) string {
	return fmt.Sprintf("%02d:%02d", clock.Hour, clock.Minute)
}
