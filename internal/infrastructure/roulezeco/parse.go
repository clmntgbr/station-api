package roulezeco

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"go-api/internal/domain/price"
	"go-api/internal/domain/station"

	"golang.org/x/text/encoding/charmap"
)

// feedTimeLayout is the clock published in prix@maj, which has no zone.
// The clock is stored as UTC so the API can render it with a Z suffix.
const feedTimeLayout = "2006-01-02 15:04:05"

// errSkipStation marks a pdv that must not become an entity.
// Invalid positions are skipped; the rest of the feed is still read.
var errSkipStation = errors.New("skip station")

type pdvXML struct {
	ID        string       `xml:"id,attr"`
	Latitude  string       `xml:"latitude,attr"`
	Longitude string       `xml:"longitude,attr"`
	CP        string       `xml:"cp,attr"`
	Pop       string       `xml:"pop,attr"`
	Address   string       `xml:"adresse"`
	City      string       `xml:"ville"`
	Hours     *hoursXML    `xml:"horaires"`
	Services  *servicesXML `xml:"services"`
	Prices    []priceXML   `xml:"prix"`
}

type servicesXML struct {
	Service []string `xml:"service"`
}

type hoursXML struct {
	Automate string   `xml:"automate-24-24,attr"`
	Days     []dayXML `xml:"jour"`
}

type dayXML struct {
	ID     int       `xml:"id,attr"`
	Name   string    `xml:"nom,attr"`
	Closed string    `xml:"ferme,attr"`
	Slots  []slotXML `xml:"horaire"`
}

type slotXML struct {
	Open  string `xml:"ouverture,attr"`
	Close string `xml:"fermeture,attr"`
}

type priceXML struct {
	Name    string `xml:"nom,attr"`
	ID      string `xml:"id,attr"`
	Updated string `xml:"maj,attr"`
	Value   string `xml:"valeur,attr"`
}

// parse reads the instant-feed XML one station at a time.
// The document is ISO-8859-1 and is not loaded whole into memory.
// Stations without a valid position are skipped.
func parse(r io.Reader, yield func(station.Station) error) error {
	if yield == nil {
		return errors.New("fuel feed yield is nil")
	}

	dec := xml.NewDecoder(r)
	dec.CharsetReader = iso88591Reader

	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("decode fuel feed: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "pdv" {
			continue
		}

		var raw pdvXML
		if err := dec.DecodeElement(&raw, &start); err != nil {
			return fmt.Errorf("decode station: %w", err)
		}
		built, err := mapPDV(raw)
		if errors.Is(err, errSkipStation) {
			continue
		}
		if err != nil {
			return fmt.Errorf("parse station %s: %w", strings.TrimSpace(raw.ID), err)
		}
		if err := yield(built); err != nil {
			return err
		}
	}
}

func iso88591Reader(charset string, input io.Reader) (io.Reader, error) {
	switch strings.ToLower(strings.TrimSpace(charset)) {
	case "iso-8859-1", "iso8859-1", "latin1", "latin-1":
		return charmap.ISO8859_1.NewDecoder().Reader(input), nil
	default:
		return nil, fmt.Errorf("unsupported xml charset %q", charset)
	}
}

func mapPDV(raw pdvXML) (station.Station, error) {
	if strings.TrimSpace(raw.ID) == "" {
		return station.Station{}, errSkipStation
	}

	lat, err := parseCoordinate(raw.Latitude)
	if err != nil {
		return station.Station{}, errSkipStation
	}
	lng, err := parseCoordinate(raw.Longitude)
	if err != nil {
		return station.Station{}, errSkipStation
	}

	hours, err := mapHours(raw.Hours)
	if err != nil {
		return station.Station{}, err
	}
	prices, err := mapPrices(raw.ID, raw.Prices)
	if err != nil {
		return station.Station{}, err
	}

	var services []string
	if raw.Services != nil {
		services = raw.Services.Service
	}

	built, err := station.New(station.Station{
		ID:         raw.ID,
		EnergyType: station.EnergyTypeFuel,
		Location:   station.Location{Latitude: lat, Longitude: lng},
		PostalCode: raw.CP,
		Address:    raw.Address,
		City:       raw.City,
		Pop:        raw.Pop,
		Hours:      hours,
		Services:   services,
		Prices:     prices,
	})
	if errors.Is(err, station.ErrInvalidLocation) {
		return station.Station{}, errSkipStation
	}
	return built, err
}

func mapHours(raw *hoursXML) (station.Hours, error) {
	if raw == nil {
		return station.Hours{}, nil
	}
	days := make([]station.Day, 0, len(raw.Days))
	for _, day := range raw.Days {
		mapped, err := mapDay(day)
		if err != nil {
			return station.Hours{}, err
		}
		days = append(days, mapped)
	}
	return station.Hours{
		Automate24: raw.Automate == "1",
		Days:       days,
	}, nil
}

func mapDay(day dayXML) (station.Day, error) {
	slots := make([]station.Slot, 0, len(day.Slots))
	for _, slot := range day.Slots {
		opens, err := parseClock(slot.Open)
		if err != nil {
			return station.Day{}, fmt.Errorf("opening time %q: %w", slot.Open, err)
		}
		closes, err := parseClock(slot.Close)
		if err != nil {
			return station.Day{}, fmt.Errorf("closing time %q: %w", slot.Close, err)
		}
		slots = append(slots, station.Slot{Opens: opens, Closes: closes})
	}
	return station.Day{
		ID:     day.ID,
		Name:   day.Name,
		Closed: day.Closed == "1",
		Slots:  slots,
	}, nil
}

func mapPrices(stationID string, raw []priceXML) ([]price.Price, error) {
	prices := make([]price.Price, 0, len(raw))
	for _, item := range raw {
		fuel, ok := fuelFromFeed(item.ID, item.Name)
		if !ok {
			return nil, fmt.Errorf("unknown fuel id %q name %q", item.ID, item.Name)
		}
		amount, err := parseAmount(item.Value)
		if err != nil {
			return nil, err
		}
		updated, err := time.ParseInLocation(feedTimeLayout, strings.TrimSpace(item.Updated), time.UTC)
		if err != nil {
			return nil, fmt.Errorf("price updated at %q: %w", item.Updated, err)
		}
		built, err := price.New(stationID, fuel, amount, updated)
		if err != nil {
			return nil, err
		}
		prices = append(prices, built)
	}
	return prices, nil
}

func fuelFromFeed(id, name string) (price.Fuel, bool) {
	switch strings.TrimSpace(id) {
	case "1":
		return price.FuelGazole, true
	case "2":
		return price.FuelSP95, true
	case "3":
		return price.FuelE85, true
	case "4":
		return price.FuelGPLc, true
	case "5":
		return price.FuelE10, true
	case "6":
		return price.FuelSP98, true
	}
	switch price.Fuel(strings.TrimSpace(name)) {
	case price.FuelGazole, price.FuelSP95, price.FuelE85, price.FuelGPLc, price.FuelE10, price.FuelSP98:
		return price.Fuel(strings.TrimSpace(name)), true
	default:
		return "", false
	}
}

// parseCoordinate converts a feed coordinate to decimal degrees.
// The feed publishes degrees multiplied by 100000 (4818300 means 48.183)
// and sometimes already includes a fractional part (4675351.71497).
// A magnitude of 180 or less is already in decimal degrees.
func parseCoordinate(raw string) (float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, errors.New("empty coordinate")
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid coordinate %q", raw)
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("invalid coordinate %q", raw)
	}
	if math.Abs(value) > 180 {
		value = value / 100_000
	}
	return value, nil
}

func parseClock(raw string) (station.Clock, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.ReplaceAll(raw, ":", ".")
	hourRaw, minuteRaw, ok := strings.Cut(raw, ".")
	if !ok {
		return station.Clock{}, fmt.Errorf("invalid time %q", raw)
	}
	hour, err := strconv.Atoi(hourRaw)
	if err != nil {
		return station.Clock{}, fmt.Errorf("invalid time %q", raw)
	}
	minute, err := strconv.Atoi(minuteRaw)
	if err != nil {
		return station.Clock{}, fmt.Errorf("invalid time %q", raw)
	}
	clock := station.Clock{Hour: hour, Minute: minute}
	if !clock.Valid() {
		return station.Clock{}, fmt.Errorf("invalid time %q", raw)
	}
	return clock, nil
}

func parseAmount(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, errors.New("empty price")
	}
	if strings.HasPrefix(raw, "-") {
		return 0, fmt.Errorf("negative price %q", raw)
	}
	whole, frac, _ := strings.Cut(raw, ".")
	if whole == "" {
		whole = "0"
	}
	if !digits(whole) || (frac != "" && !digits(frac)) {
		return 0, fmt.Errorf("invalid price %q", raw)
	}
	roundUp := false
	if len(frac) > 3 {
		roundUp = frac[3] >= '5'
		frac = frac[:3]
	}
	for len(frac) < 3 {
		frac += "0"
	}
	units, err := strconv.Atoi(whole)
	if err != nil {
		return 0, fmt.Errorf("invalid price %q", raw)
	}
	millis, err := strconv.Atoi(frac)
	if err != nil {
		return 0, fmt.Errorf("invalid price %q", raw)
	}
	amount := units*1000 + millis
	if roundUp {
		amount++
	}
	if amount > 999999 {
		return 0, fmt.Errorf("price out of range %q", raw)
	}
	return amount, nil
}

func digits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
