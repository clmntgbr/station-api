package roulezeco

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"go-api/internal/domain/station"
)

const (
	// DefaultFeedURL is the roulez-eco instant fuel-price archive.
	DefaultFeedURL = "https://donnees.roulez-eco.fr/opendata/instantane"

	defaultMaxFeedBytes int64 = 64 << 20
)

// Feed downloads the instant archive and streams one station entity at a time.
type Feed struct {
	url        string
	httpClient *http.Client
	maxBytes   int64
}

// NewFeed builds a feed reader. An empty URL uses DefaultFeedURL.
// A nil HTTP client uses a two-minute timeout.
func NewFeed(feedURL string, httpClient *http.Client) *Feed {
	if feedURL == "" {
		feedURL = DefaultFeedURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 2 * time.Minute}
	}
	return &Feed{url: feedURL, httpClient: httpClient}
}

// Stream downloads the ZIP and calls yield for each station that has a valid position.
// yield's error stops the read and is returned as-is.
func (f *Feed) Stream(ctx context.Context, yield func(station.Station) error) error {
	if yield == nil {
		return fmt.Errorf("fuel feed yield is nil")
	}
	path, err := f.download(ctx)
	if err != nil {
		return err
	}
	defer os.Remove(path)

	return streamZip(path, func(r io.Reader) error {
		return parse(r, func(s station.Station) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			return yield(s)
		})
	})
}

func (f *Feed) download(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
	if err != nil {
		return "", fmt.Errorf("build fuel feed request: %w", err)
	}
	req.Header.Set("User-Agent", "station-api")
	req.Header.Set("Accept", "application/zip, application/octet-stream")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download fuel feed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download fuel feed: status %s", resp.Status)
	}

	tmp, err := os.CreateTemp("", "roulez-eco-*.zip")
	if err != nil {
		return "", fmt.Errorf("create fuel feed file: %w", err)
	}
	path := tmp.Name()

	limit := f.maxBytes
	if limit <= 0 {
		limit = defaultMaxFeedBytes
	}
	n, err := io.Copy(tmp, io.LimitReader(resp.Body, limit+1))
	if err != nil {
		discardTemp(tmp)
		return "", fmt.Errorf("save fuel feed: %w", err)
	}
	if n > limit {
		discardTemp(tmp)
		return "", fmt.Errorf("fuel feed exceeds %d bytes", limit)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("close fuel feed file: %w", err)
	}
	return path, nil
}

func discardTemp(f *os.File) {
	if f == nil {
		return
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
}

func streamZip(path string, read func(io.Reader) error) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("open fuel feed zip: %w", err)
	}
	defer zr.Close()

	entry := xmlFile(zr)
	if entry == nil {
		return fmt.Errorf("fuel feed zip has no xml file")
	}
	rc, err := entry.Open()
	if err != nil {
		return fmt.Errorf("open fuel feed xml: %w", err)
	}
	defer rc.Close()
	return read(rc)
}

func xmlFile(zr *zip.ReadCloser) *zip.File {
	for _, file := range zr.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(file.Name), ".xml") {
			return file
		}
	}
	return nil
}
