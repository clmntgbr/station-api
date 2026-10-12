package write

import (
	"encoding/json"
	"fmt"

	"go-api/internal/domain/station"
	"go-api/internal/infrastructure/persistence/dbtype"

	"github.com/google/uuid"
)

// StationModel is the persistence mapping for table stations.
// Schema is owned by SQL migrations — do not encode DDL in tags.
// The ingester only reads info_hash and price_hash. The worker owns writes.
// Latitude and longitude are WGS84 decimal degrees.
type StationModel struct {
	ID            uuid.UUID    `gorm:"column:id;primaryKey"`
	StationID     string       `gorm:"column:station_id"`
	EnergyType    string       `gorm:"column:energy_type"`
	Latitude      float64      `gorm:"column:latitude"`
	Longitude     float64      `gorm:"column:longitude"`
	PostalCode    string       `gorm:"column:postal_code"`
	Address       string       `gorm:"column:address"`
	City          string       `gorm:"column:city"`
	Pop           string       `gorm:"column:pop"`
	Automate24    bool         `gorm:"column:automate_24"`
	Hours         dbtype.JSONB `gorm:"column:hours"`
	Services      dbtype.JSONB `gorm:"column:services"`
	InfoHash      string       `gorm:"column:info_hash"`
	PriceHash     string       `gorm:"column:price_hash"`
	GooglePlaceID *string      `gorm:"column:google_place_id"`
}

func (StationModel) TableName() string {
	return "stations"
}

type hoursJSON struct {
	Days []dayJSON `json:"days"`
}

type dayJSON struct {
	ID     int        `json:"id"`
	Name   string     `json:"name"`
	Closed bool       `json:"closed"`
	Slots  []slotJSON `json:"slots"`
}

type slotJSON struct {
	Open  string `json:"open"`
	Close string `json:"close"`
}

func stationModelFromDomain(s station.Station, syncPrices bool) (StationModel, error) {
	days := make([]dayJSON, len(s.Hours.Days))
	for i, day := range s.Hours.Days {
		slots := make([]slotJSON, len(day.Slots))
		for j, slot := range day.Slots {
			slots[j] = slotJSON{
				Open:  fmt.Sprintf("%02d:%02d", slot.Opens.Hour, slot.Opens.Minute),
				Close: fmt.Sprintf("%02d:%02d", slot.Closes.Hour, slot.Closes.Minute),
			}
		}
		days[i] = dayJSON{
			ID:     day.ID,
			Name:   day.Name,
			Closed: day.Closed,
			Slots:  slots,
		}
	}
	hours, err := json.Marshal(hoursJSON{Days: days})
	if err != nil {
		return StationModel{}, fmt.Errorf("encode hours: %w", err)
	}

	services := s.Services
	if services == nil {
		services = []string{}
	}
	encodedServices, err := json.Marshal(services)
	if err != nil {
		return StationModel{}, fmt.Errorf("encode services: %w", err)
	}
	priceHash := ""
	if syncPrices {
		priceHash = s.PriceHash()
	}

	return StationModel{
		ID:         s.ID,
		StationID:  s.StationID,
		EnergyType: string(s.EnergyType),
		Latitude:   s.Location.Latitude,
		Longitude:  s.Location.Longitude,
		PostalCode: s.PostalCode,
		Address:    s.Address,
		City:       s.City,
		Pop:        s.Pop,
		Automate24: s.Hours.Automate24,
		Hours:      hours,
		Services:   encodedServices,
		InfoHash:   s.InfoHash(),
		PriceHash:  priceHash,
	}, nil
}
