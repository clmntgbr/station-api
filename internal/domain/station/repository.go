package station

import (
	"context"
	"time"

	"go-api/internal/domain/price"

	"github.com/google/uuid"
)

// Hashes are the stored fingerprints of one station.
// ID is the internal UUID v7. The map returned by HashReader is keyed by the feed station id.
type Hashes struct {
	ID    uuid.UUID
	Info  string
	Price string
}

// HashReader loads the stored info and price hashes of every station.
// The map key is the feed station id.
type HashReader interface {
	FindHashes(ctx context.Context) (map[string]Hashes, error)
}

// WriteRepository persists a station snapshot and its current prices.
// Upsert with syncPrices also stores the price hash; otherwise it leaves that column untouched.
// Exists reports whether the station row is already there for the price task.
type WriteRepository interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	Upsert(ctx context.Context, station Station, syncPrices bool) (uuid.UUID, error)
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
	FindPrices(ctx context.Context, id uuid.UUID) ([]price.Price, error)
	SyncPrices(ctx context.Context, id uuid.UUID, prices []price.Price, changes []PriceChange, observedAt time.Time, priceHash string) error
}
