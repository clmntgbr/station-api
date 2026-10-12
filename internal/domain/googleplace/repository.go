package googleplace

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// WriteRepository persists google place snapshots.
type WriteRepository interface {
	WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error
	Save(ctx context.Context, place GooglePlace) error
	Update(ctx context.Context, place GooglePlace) error
	GetByID(ctx context.Context, id uuid.UUID) (GooglePlace, error)
	GetByPlaceID(ctx context.Context, placeID string) (GooglePlace, error)
}

// ReadRepository loads google place read models.
type ReadRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*GooglePlaceView, error)
	FindByPlaceID(ctx context.Context, placeID string) (*GooglePlaceView, error)
}

// GooglePlaceView is the read-side projection of a google place.
type GooglePlaceView struct {
	ID                       uuid.UUID
	PlaceID                  string
	InternationalPhoneNumber string
	Rating                   *float64
	UserRatingCount          *int
	BusinessStatus           string
	WebsiteURL               string
	GoogleMapDirectionsURL   string
	GoogleMapURL             string
	CreatedAt                time.Time
	UpdatedAt                time.Time
}
