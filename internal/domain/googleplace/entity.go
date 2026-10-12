package googleplace

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// BusinessStatusOperational means the place is open for business.
	BusinessStatusOperational = "OPERATIONAL"
	// BusinessStatusClosedTemporarily means the place is temporarily closed.
	BusinessStatusClosedTemporarily = "CLOSED_TEMPORARILY"
	// BusinessStatusClosedPermanently means the place is permanently closed.
	BusinessStatusClosedPermanently = "CLOSED_PERMANENTLY"

	minRating = 0.0
	maxRating = 5.0
)

// ErrInvalidGooglePlace means the place identity or attributes cannot be stored.
var ErrInvalidGooglePlace = errors.New("invalid google place")

// GooglePlace is enrichment data from Google Places for one location.
// PlaceID is the durable Google identifier; other fields are a refreshable snapshot.
type GooglePlace struct {
	// ID is the internal primary key, a UUID v7 assigned when the place is first stored.
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

// New validates a google place and returns a canonical copy.
// When ID is uuid.Nil, a UUID v7 is assigned. CreatedAt / UpdatedAt default to now UTC when zero.
func New(p GooglePlace) (GooglePlace, error) {
	p.PlaceID = strings.TrimSpace(p.PlaceID)
	p.InternationalPhoneNumber = strings.TrimSpace(p.InternationalPhoneNumber)
	p.BusinessStatus = strings.TrimSpace(p.BusinessStatus)
	p.WebsiteURL = strings.TrimSpace(p.WebsiteURL)
	p.GoogleMapDirectionsURL = strings.TrimSpace(p.GoogleMapDirectionsURL)
	p.GoogleMapURL = strings.TrimSpace(p.GoogleMapURL)

	if p.PlaceID == "" {
		return GooglePlace{}, ErrInvalidGooglePlace
	}
	if p.ID == uuid.Nil {
		id, err := uuid.NewV7()
		if err != nil {
			return GooglePlace{}, ErrInvalidGooglePlace
		}
		p.ID = id
	} else if p.ID.Version() != 7 {
		return GooglePlace{}, ErrInvalidGooglePlace
	}

	if p.Rating != nil {
		rating := *p.Rating
		if rating < minRating || rating > maxRating {
			return GooglePlace{}, ErrInvalidGooglePlace
		}
		p.Rating = &rating
	}
	if p.UserRatingCount != nil {
		count := *p.UserRatingCount
		if count < 0 {
			return GooglePlace{}, ErrInvalidGooglePlace
		}
		p.UserRatingCount = &count
	}
	if p.BusinessStatus != "" && !ValidBusinessStatus(p.BusinessStatus) {
		return GooglePlace{}, ErrInvalidGooglePlace
	}

	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	} else {
		p.CreatedAt = p.CreatedAt.UTC()
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = now
	} else {
		p.UpdatedAt = p.UpdatedAt.UTC()
	}

	return p, nil
}

// ValidBusinessStatus reports whether status is a known Google Places business status.
func ValidBusinessStatus(status string) bool {
	switch status {
	case BusinessStatusOperational, BusinessStatusClosedTemporarily, BusinessStatusClosedPermanently:
		return true
	default:
		return false
	}
}
