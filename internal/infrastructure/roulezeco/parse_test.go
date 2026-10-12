package roulezeco

import (
	"bytes"
	"math"
	"testing"
	"time"

	"go-api/internal/domain/price"
	"go-api/internal/domain/station"

	"golang.org/x/text/encoding/charmap"
)

const sampleXML = `<?xml version="1.0" encoding="ISO-8859-1" standalone="yes"?>
<pdv_liste>
  <pdv id="89100001" latitude="4818300" longitude="330900" cp="89100" pop="R">
    <adresse>84 ROUTE DE MAILLOT</adresse>
    <ville>SENS</ville>
    <horaires automate-24-24="">
      <jour id="1" nom="Lundi" ferme=""/>
    </horaires>
    <services>
      <service>Toilettes publiques</service>
      <service>Wifi</service>
    </services>
    <prix nom="SP98" id="6" maj="2026-09-28 11:42:00" valeur="2.289"/>
    <prix nom="Gazole" id="1" maj="2026-10-11 08:50:09" valeur="2.399"/>
    <prix nom="E10" id="5" maj="2026-10-11 08:50:09" valeur="2.229"/>
    <prix nom="E85" id="3" maj="2026-10-05 12:16:34" valeur="0.909"/>
  </pdv>
  <pdv id="58240003" latitude="4675351.71497" longitude="314069.121289" cp="58240" pop="R">
    <adresse>ROUTE NATIONALE 7 - LES BOULAISES</adresse>
    <ville>Chantenay-Saint-Imbert</ville>
    <horaires automate-24-24="">
      <jour id="1" nom="Lundi" ferme=""/>
    </horaires>
    <services/>
  </pdv>
  <pdv id="34150003" latitude="4365100" longitude="354700" cp="34150" pop="R">
    <adresse>Route de Pézenas</adresse>
    <ville>GIGNAC</ville>
    <horaires automate-24-24="1">
      <jour id="1" nom="Lundi" ferme="1">
        <horaire ouverture="01.00" fermeture="01.00"/>
      </jour>
      <jour id="2" nom="Mardi" ferme="">
        <horaire ouverture="08.00" fermeture="12.00"/>
        <horaire ouverture="14.00" fermeture="19.00"/>
      </jour>
    </horaires>
    <services>
      <service>Station de gonflage</service>
      <service>Lavage automatique</service>
    </services>
    <prix nom="Gazole" id="1" maj="2026-10-12 08:30:46" valeur="2.334"/>
  </pdv>
  <pdv id="1000001" latitude="48.183" longitude="3.309" cp="01000" pop="A">
    <adresse>596 AVENUE DE TREVOUX</adresse>
    <ville>SAINT-DENIS</ville>
    <services>
      <service>Boutique alimentaire</service>
    </services>
    <prix nom="SP95" id="2" maj="2026-10-01 10:00:00" valeur="1.899"/>
    <prix nom="GPLc" id="4" maj="2026-10-01 10:00:00" valeur="0.999"/>
  </pdv>
  <pdv id="00000000" latitude="9100000" longitude="354700" cp="00000" pop="R">
    <adresse>Nulle part</adresse>
    <ville>NULLE</ville>
    <services/>
  </pdv>
  <pdv id="00000001" latitude="" longitude="" cp="00000" pop="R">
    <adresse>Vide</adresse>
    <ville>VIDE</ville>
  </pdv>
</pdv_liste>
`

func TestParseFeedQuirks(t *testing.T) {
	encoded := encodeLatin1(t, sampleXML)
	if bytes.Contains(encoded, []byte("Pézenas")) {
		t.Fatal("fixture is still UTF-8")
	}
	if !bytes.Contains(encoded, []byte{0xE9}) {
		t.Fatal("fixture has no ISO-8859-1 e acute")
	}

	first := indexStations(t, encoded)
	second := indexStations(t, encoded)
	if len(first) != 4 {
		t.Fatalf("got %d stations, want 4", len(first))
	}
	if _, skipped := first["00000000"]; skipped {
		t.Fatal("station with latitude 91 was kept")
	}
	if _, skipped := first["00000001"]; skipped {
		t.Fatal("station without coordinates was kept")
	}

	sens := first["89100001"]
	if math.Abs(sens.Location.Latitude-48.183) > 1e-9 || math.Abs(sens.Location.Longitude-3.309) > 1e-9 {
		t.Fatalf("sens coordinates = %v, %v", sens.Location.Latitude, sens.Location.Longitude)
	}
	if sens.Hours.Automate24 || sens.Hours.Days[0].Closed || len(sens.Hours.Days[0].Slots) != 0 {
		t.Fatalf("sens hours = %#v", sens.Hours)
	}
	if len(sens.Prices) != 4 {
		t.Fatalf("sens prices = %#v", sens.Prices)
	}
	if sens.Prices[0].Fuel != price.FuelGazole || sens.Prices[0].Amount != 2399 {
		t.Fatalf("first price = %#v", sens.Prices[0])
	}
	if sens.Prices[1].Fuel != price.FuelE85 || sens.Prices[1].Amount != 909 {
		t.Fatalf("e85 = %#v", sens.Prices[1])
	}
	gazoleAt := time.Date(2026, 10, 11, 8, 50, 9, 0, time.UTC)
	sp98At := time.Date(2026, 9, 28, 11, 42, 0, 0, time.UTC)
	if !sens.Prices[0].SourceUpdatedAt.Equal(gazoleAt) || !sens.Prices[3].SourceUpdatedAt.Equal(sp98At) {
		t.Fatalf("price dates = %s, %s", sens.Prices[0].SourceUpdatedAt, sens.Prices[3].SourceUpdatedAt)
	}
	if sens.Services[0] != "Toilettes publiques" || sens.Services[1] != "Wifi" {
		t.Fatalf("services = %#v", sens.Services)
	}

	empty := first["58240003"]
	if math.Abs(empty.Location.Latitude-46.7535171497) > 1e-8 || math.Abs(empty.Location.Longitude-3.14069121289) > 1e-8 {
		t.Fatalf("decimal coordinates = %v, %v", empty.Location.Latitude, empty.Location.Longitude)
	}
	if len(empty.Prices) != 0 || empty.Prices == nil || len(empty.Services) != 0 || empty.Services == nil {
		t.Fatalf("empty station prices=%#v services=%#v", empty.Prices, empty.Services)
	}

	gignac := first["34150003"]
	if gignac.Address != "Route de Pézenas" || gignac.City != "GIGNAC" {
		t.Fatalf("gignac address = %q %q", gignac.Address, gignac.City)
	}
	if math.Abs(gignac.Location.Latitude-43.651) > 1e-9 || math.Abs(gignac.Location.Longitude-3.547) > 1e-9 {
		t.Fatalf("gignac coordinates = %v, %v", gignac.Location.Latitude, gignac.Location.Longitude)
	}
	if !gignac.Hours.Automate24 {
		t.Fatal("gignac automate flag not set")
	}
	monday := gignac.Hours.Days[0]
	if monday.ID != 1 || !monday.Closed || monday.Slots[0].Opens != (station.Clock{Hour: 1}) || monday.Slots[0].Closes != (station.Clock{Hour: 1}) {
		t.Fatalf("monday = %#v", monday)
	}
	tuesday := gignac.Hours.Days[1]
	if len(tuesday.Slots) != 2 || tuesday.Slots[0].Closes != (station.Clock{Hour: 12}) || tuesday.Slots[1].Opens != (station.Clock{Hour: 14}) {
		t.Fatalf("tuesday = %#v", tuesday)
	}
	if gignac.Prices[0].Amount != 2334 {
		t.Fatalf("gignac price = %#v", gignac.Prices[0])
	}
	if gignac.InfoHash() == "" || gignac.InfoHash() != second["34150003"].InfoHash() || gignac.PriceHash() != second["34150003"].PriceHash() {
		t.Fatal("hashes are not stable across parses")
	}

	motorway := first["1000001"]
	if motorway.Pop != station.PopMotorway || len(motorway.Hours.Days) != 0 {
		t.Fatalf("motorway = %#v", motorway)
	}
	if motorway.PostalCode != "01000" {
		t.Fatalf("postal code = %q", motorway.PostalCode)
	}
	if motorway.Prices[0].Fuel != price.FuelSP95 || motorway.Prices[0].Amount != 1899 || motorway.Prices[1].Fuel != price.FuelGPLc || motorway.Prices[1].Amount != 999 {
		t.Fatalf("motorway prices = %#v", motorway.Prices)
	}
	if math.Abs(motorway.Location.Latitude-48.183) > 1e-9 {
		t.Fatalf("decimal degrees were rescaled: %v", motorway.Location.Latitude)
	}
}

func TestParseCoordinate(t *testing.T) {
	cases := []struct {
		raw     string
		want    float64
		wantErr bool
	}{
		{raw: "4818300", want: 48.183},
		{raw: "330900", want: 3.309},
		{raw: "4675351.71497", want: 46.7535171497},
		{raw: "314069.121289", want: 3.14069121289},
		{raw: "43.651", want: 43.651},
		{raw: "-523000", want: -5.23},
		{raw: "180", want: 180},
		{raw: "", wantErr: true},
		{raw: "abc", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := parseCoordinate(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(got-tc.want) > 1e-8 {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseAmount(t *testing.T) {
	cases := map[string]int{
		"2.399":  2399,
		"0.909":  909,
		"2.334":  2334,
		"2":      2000,
		"2.2":    2200,
		".909":   909,
		"2.3996": 2400,
	}
	for raw, want := range cases {
		got, err := parseAmount(raw)
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		if got != want {
			t.Fatalf("%s: got %d, want %d", raw, got, want)
		}
	}
	if _, err := parseAmount("-1.000"); err == nil {
		t.Fatal("negative price accepted")
	}
	if _, err := parseAmount("abc"); err == nil {
		t.Fatal("invalid price accepted")
	}
}

func TestParseClock(t *testing.T) {
	got, err := parseClock("00.00")
	if err != nil || got != (station.Clock{}) {
		t.Fatalf("midnight = %#v, %v", got, err)
	}
	got, err = parseClock("08.30")
	if err != nil || got != (station.Clock{Hour: 8, Minute: 30}) {
		t.Fatalf("08.30 = %#v, %v", got, err)
	}
	got, err = parseClock("07.15")
	if err != nil || got != (station.Clock{Hour: 7, Minute: 15}) {
		t.Fatalf("07.15 = %#v, %v", got, err)
	}
	if _, err := parseClock("24.00"); err == nil {
		t.Fatal("24.00 accepted")
	}
}

func TestFuelFromFeedIDWins(t *testing.T) {
	fuel, ok := fuelFromFeed("1", "SP98")
	if !ok || fuel != price.FuelGazole {
		t.Fatalf("id did not win: %s %v", fuel, ok)
	}
	fuel, ok = fuelFromFeed("", "E10")
	if !ok || fuel != price.FuelE10 {
		t.Fatalf("name fallback = %s %v", fuel, ok)
	}
	if _, ok := fuelFromFeed("9", "Inconnu"); ok {
		t.Fatal("unknown fuel accepted")
	}
}

func TestParseUnknownFuel(t *testing.T) {
	xmlDoc := encodeLatin1(t, `<?xml version="1.0" encoding="ISO-8859-1"?>
<pdv_liste>
  <pdv id="1" latitude="4818300" longitude="330900" cp="89100" pop="R">
    <adresse>Rue</adresse>
    <ville>SENS</ville>
    <prix nom="Inconnu" id="9" maj="2026-10-11 08:50:09" valeur="1.000"/>
  </pdv>
</pdv_liste>`)
	err := parse(bytes.NewReader(xmlDoc), func(station.Station) error { return nil })
	if err == nil {
		t.Fatal("unknown fuel did not fail the parse")
	}
}

func indexStations(t *testing.T, encoded []byte) map[string]station.Station {
	t.Helper()
	out := map[string]station.Station{}
	err := parse(bytes.NewReader(encoded), func(s station.Station) error {
		out[s.StationID] = s
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func encodeLatin1(t *testing.T, utf8 string) []byte {
	t.Helper()
	out, err := charmap.ISO8859_1.NewEncoder().Bytes([]byte(utf8))
	if err != nil {
		t.Fatal(err)
	}
	return out
}
