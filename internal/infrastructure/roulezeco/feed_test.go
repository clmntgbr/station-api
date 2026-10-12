package roulezeco

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"go-api/internal/domain/station"
)

func TestFeedStream(t *testing.T) {
	payload := zipBytes(t, map[string][]byte{
		"PrixCarburants_instantane.xml": encodeLatin1(t, sampleXML),
	})
	var userAgent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	feed := NewFeed(srv.URL, srv.Client())
	var ids []string
	err := feed.Stream(context.Background(), func(s station.Station) error {
		ids = append(ids, s.StationID)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if userAgent != "station-api" {
		t.Fatalf("user agent = %q", userAgent)
	}
	if len(ids) != 4 || ids[2] != "34150003" {
		t.Fatalf("ids = %#v", ids)
	}
}

func TestFeedStreamStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	feed := NewFeed(srv.URL, srv.Client())
	err := feed.Stream(context.Background(), func(station.Station) error { return nil })
	if err == nil {
		t.Fatal("expected status error")
	}
}

func TestFeedStreamTooLarge(t *testing.T) {
	payload := zipBytes(t, map[string][]byte{
		"PrixCarburants_instantane.xml": encodeLatin1(t, sampleXML),
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	feed := NewFeed(srv.URL, srv.Client())
	feed.maxBytes = 8
	err := feed.Stream(context.Background(), func(station.Station) error { return nil })
	if err == nil {
		t.Fatal("expected size error")
	}
}

func TestFeedStreamYieldError(t *testing.T) {
	payload := zipBytes(t, map[string][]byte{
		"PrixCarburants_instantane.xml": encodeLatin1(t, sampleXML),
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	feed := NewFeed(srv.URL, srv.Client())
	err := feed.Stream(context.Background(), func(station.Station) error {
		return errors.New("stop")
	})
	if err == nil || err.Error() != "stop" {
		t.Fatalf("got %v", err)
	}
}

func TestFeedStreamCanceled(t *testing.T) {
	feed := NewFeed("http://example.invalid", nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := feed.Stream(ctx, func(station.Station) error { return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestFeedStreamZipWithoutXML(t *testing.T) {
	payload := zipBytes(t, map[string][]byte{"readme.txt": []byte("nope")})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	feed := NewFeed(srv.URL, srv.Client())
	err := feed.Stream(context.Background(), func(station.Station) error { return nil })
	if err == nil {
		t.Fatal("expected missing xml error")
	}
}

func TestStreamRealFeed(t *testing.T) {
	zipPath := os.Getenv("ROULEZ_ECO_ZIP")
	if zipPath == "" {
		t.Skip("ROULEZ_ECO_ZIP is not set")
	}

	var (
		count         int
		withoutPrices int
		gignac        *station.Station
		sens          *station.Station
	)
	err := streamZip(zipPath, func(r io.Reader) error {
		return parse(r, func(s station.Station) error {
			count++
			if len(s.Prices) == 0 {
				withoutPrices++
			}
			if s.Location.Latitude < 41 || s.Location.Latitude > 52 || s.Location.Longitude < -6 || s.Location.Longitude > 11 {
				t.Errorf("station %s outside metropolitan France: %v", s.StationID, s.Location)
			}
			switch s.StationID {
			case "34150003":
				copied := s
				gignac = &copied
			case "89100001":
				copied := s
				sens = &copied
			}
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	if count < 9000 {
		t.Fatalf("parsed %d stations", count)
	}
	if withoutPrices == 0 {
		t.Fatal("expected stations without prices")
	}
	if gignac == nil || gignac.Address != "Route de Pézenas" || gignac.City != "GIGNAC" {
		t.Fatalf("gignac = %#v", gignac)
	}
	if sens == nil || len(sens.Prices) < 2 {
		t.Fatalf("sens = %#v", sens)
	}
}

func zipBytes(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
