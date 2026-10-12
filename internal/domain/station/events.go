package station

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"go-api/internal/domain/event"
	"go-api/internal/domain/price"

	"github.com/google/uuid"
)

const (
	EventTypeStationUpserted = "station.upserted.v1"
	EventTypePricesUpdated   = "station.prices_updated.v1"
	EventTypePriceChanged    = "station.price_changed.v1"
)

// StationUpserted is the payload of station.upserted.v1.
// EventID is the dedup key the worker stores with the write.
// InfoHash is the fingerprint of the station without its prices.
// SyncPrices is set only when this message creates the station: the price task
// cannot run first, because the station row does not exist yet.
// Amount on each price is thousandths of a euro per litre.
type StationUpserted struct {
	EventID    string               `json:"eventId"`
	ID         uuid.UUID            `json:"id"`
	StationID  string               `json:"stationId"`
	EnergyType EnergyType           `json:"energyType"`
	Latitude   float64              `json:"latitude"`
	Longitude  float64              `json:"longitude"`
	PostalCode string               `json:"postalCode"`
	Address    string               `json:"address"`
	City       string               `json:"city"`
	Pop        string               `json:"pop"`
	Automate24 bool                 `json:"automate24"`
	Days       []StationUpsertDay   `json:"days"`
	Services   []string             `json:"services"`
	Prices     []StationUpsertPrice `json:"prices,omitempty"`
	InfoHash   string               `json:"infoHash"`
	PriceHash  string               `json:"priceHash,omitempty"`
	SyncPrices bool                 `json:"syncPrices,omitempty"`
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

// NewStationUpserted builds the station task.
// syncPrices includes the prices: the worker writes them while creating the station.
func NewStationUpserted(s Station, syncPrices bool) StationUpserted {
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
	upserted := StationUpserted{
		ID:         s.ID,
		StationID:  s.StationID,
		EnergyType: s.EnergyType,
		Latitude:   s.Location.Latitude,
		Longitude:  s.Location.Longitude,
		PostalCode: s.PostalCode,
		Address:    s.Address,
		City:       s.City,
		Pop:        s.Pop,
		Automate24: s.Hours.Automate24,
		Days:       days,
		Services:   services,
		InfoHash:   s.InfoHash(),
		SyncPrices: syncPrices,
	}
	if syncPrices {
		upserted.Prices = upsertPrices(s)
		upserted.PriceHash = s.PriceHash()
	}
	return upserted
}

func upsertPrices(s Station) []StationUpsertPrice {
	prices := make([]StationUpsertPrice, len(s.Prices))
	for i, item := range s.Prices {
		prices[i] = StationUpsertPrice{
			StationID:       item.StationID,
			Fuel:            item.Fuel,
			Amount:          item.Amount,
			SourceUpdatedAt: item.SourceUpdatedAt.UTC(),
		}
	}
	return prices
}

// PricesUpdated is the payload of station.prices_updated.v1.
// The ingester publishes it only when the station row already exists.
type PricesUpdated struct {
	EventID   string               `json:"eventId"`
	ID        uuid.UUID            `json:"id"`
	StationID string               `json:"stationId"`
	Prices    []StationUpsertPrice `json:"prices"`
	PriceHash string               `json:"priceHash"`
}

// NewPricesUpdated builds the price task from a station.
func NewPricesUpdated(s Station) PricesUpdated {
	prices := upsertPrices(s)
	if prices == nil {
		prices = []StationUpsertPrice{}
	}
	return PricesUpdated{
		ID:        s.ID,
		StationID: s.StationID,
		Prices:    prices,
		PriceHash: s.PriceHash(),
	}
}

// StationPrices rebuilds the priced snapshot used to check PriceHash.
func (e PricesUpdated) StationPrices() (Station, error) {
	raw := make([]price.Price, len(e.Prices))
	for i, item := range e.Prices {
		stationID := strings.TrimSpace(item.StationID)
		if stationID == "" {
			stationID = e.StationID
		}
		raw[i] = price.Price{
			StationID:       stationID,
			Fuel:            item.Fuel,
			Amount:          item.Amount,
			SourceUpdatedAt: item.SourceUpdatedAt,
		}
	}
	canonical, err := price.Canonical(raw)
	if err != nil {
		return Station{}, err
	}
	for _, item := range canonical {
		if item.StationID != e.StationID {
			return Station{}, price.ErrInvalidPrice
		}
	}
	if e.ID == uuid.Nil || e.ID.Version() != 7 {
		return Station{}, ErrInvalidStation
	}
	return Station{ID: e.ID, StationID: e.StationID, Prices: canonical}, nil
}

func formatClock(clock Clock) string {
	return fmt.Sprintf("%02d:%02d", clock.Hour, clock.Minute)
}

// Station rebuilds the domain station from the published payload.
func (e StationUpserted) Station() (Station, error) {
	days := make([]Day, len(e.Days))
	for i, day := range e.Days {
		slots := make([]Slot, len(day.Slots))
		for j, slot := range day.Slots {
			opens, err := parseClock(slot.Open)
			if err != nil {
				return Station{}, err
			}
			closes, err := parseClock(slot.Close)
			if err != nil {
				return Station{}, err
			}
			slots[j] = Slot{Opens: opens, Closes: closes}
		}
		days[i] = Day{
			ID:     day.ID,
			Name:   day.Name,
			Closed: day.Closed,
			Slots:  slots,
		}
	}

	prices := make([]price.Price, len(e.Prices))
	for i, item := range e.Prices {
		prices[i] = price.Price{
			StationID:       item.StationID,
			Fuel:            item.Fuel,
			Amount:          item.Amount,
			SourceUpdatedAt: item.SourceUpdatedAt,
		}
	}

	if e.ID == uuid.Nil || e.ID.Version() != 7 {
		return Station{}, ErrInvalidStation
	}
	return New(Station{
		ID:         e.ID,
		StationID:  e.StationID,
		EnergyType: e.EnergyType,
		Location:   Location{Latitude: e.Latitude, Longitude: e.Longitude},
		PostalCode: e.PostalCode,
		Address:    e.Address,
		City:       e.City,
		Pop:        e.Pop,
		Hours:      Hours{Automate24: e.Automate24, Days: days},
		Services:   e.Services,
		Prices:     prices,
	})
}

func parseClock(value string) (Clock, error) {
	hourText, minuteText, ok := strings.Cut(value, ":")
	if !ok || len(hourText) != 2 || len(minuteText) != 2 {
		return Clock{}, ErrInvalidStation
	}
	hour, err := strconv.Atoi(hourText)
	if err != nil {
		return Clock{}, ErrInvalidStation
	}
	minute, err := strconv.Atoi(minuteText)
	if err != nil {
		return Clock{}, ErrInvalidStation
	}
	clock := Clock{Hour: hour, Minute: minute}
	if !clock.Valid() {
		return Clock{}, ErrInvalidStation
	}
	return clock, nil
}

// PriceChanged is the payload of station.price_changed.v1.
// Amounts are thousandths of a euro per litre.
// PreviousAmount is absent when the fuel is new at the station.
type PriceChanged struct {
	ID              string     `json:"eventId"`
	StationUUID     uuid.UUID  `json:"id"`
	StationID       string     `json:"stationId"`
	Fuel            price.Fuel `json:"fuel"`
	PreviousAmount  *int       `json:"previousAmount"`
	Amount          int        `json:"amount"`
	SourceUpdatedAt time.Time  `json:"sourceUpdatedAt"`
	ObservedAt      time.Time  `json:"occurredAt"`
}

func (e PriceChanged) EventID() string       { return e.ID }
func (e PriceChanged) EventType() string     { return EventTypePriceChanged }
func (e PriceChanged) AggregateID() string   { return e.StationUUID.String() }
func (e PriceChanged) OccurredAt() time.Time { return e.ObservedAt }

// NewPriceChanged builds a stable event for one fuel whose amount changed.
func NewPriceChanged(stationUUID uuid.UUID, change PriceChange, occurredAt time.Time) PriceChanged {
	occurredAt = occurredAt.UTC()
	previous := ""
	if change.PreviousAmount != nil {
		previous = strconv.Itoa(*change.PreviousAmount)
	}
	return PriceChanged{
		ID: event.DeterministicID(
			EventTypePriceChanged,
			change.Price.StationID,
			string(change.Price.Fuel),
			previous,
			strconv.Itoa(change.Price.Amount),
			change.Price.SourceUpdatedAt.UTC().Format(time.RFC3339Nano),
		),
		StationUUID:     stationUUID,
		StationID:       change.Price.StationID,
		Fuel:            change.Price.Fuel,
		PreviousAmount:  change.PreviousAmount,
		Amount:          change.Price.Amount,
		SourceUpdatedAt: change.Price.SourceUpdatedAt.UTC(),
		ObservedAt:      occurredAt,
	}
}
