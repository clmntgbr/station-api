package station

import "context"

// ContentHashReader loads the stored content hash of every station.
// A missing id means the station has never been written.
type ContentHashReader interface {
	FindContentHashes(ctx context.Context) (map[string]string, error)
}
