package googleplace

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewRejectsInvalidGooglePlace(t *testing.T) {
	rating := 4.2
	count := 12
	base := GooglePlace{
		PlaceID:                  "ChIJN1t_tDeuEmsRUsoyG83frY4",
		InternationalPhoneNumber: "+33 1 23 45 67 89",
		Rating:                   &rating,
		UserRatingCount:          &count,
		BusinessStatus:           BusinessStatusOperational,
		WebsiteURL:               "https://example.com",
		GoogleMapDirectionsURL:   "https://maps.google.com/dir",
		GoogleMapURL:             "https://maps.google.com/place",
	}

	badRating := 5.1
	negCount := -1

	cases := []struct {
		name string
		edit func(*GooglePlace)
	}{
		{name: "empty place id", edit: func(p *GooglePlace) { p.PlaceID = " " }},
		{name: "uuid version", edit: func(p *GooglePlace) {
			p.ID = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
		}},
		{name: "rating too high", edit: func(p *GooglePlace) { p.Rating = &badRating }},
		{name: "negative rating count", edit: func(p *GooglePlace) { p.UserRatingCount = &negCount }},
		{name: "unknown business status", edit: func(p *GooglePlace) { p.BusinessStatus = "UNKNOWN" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := base
			tc.edit(&input)

			_, err := New(input)
			if !errors.Is(err, ErrInvalidGooglePlace) {
				t.Fatalf("got %v, want %v", err, ErrInvalidGooglePlace)
			}
		})
	}
}

func TestNewCanonicalizesGooglePlace(t *testing.T) {
	rating := 4.5
	count := 42
	created := time.Date(2026, 10, 1, 10, 0, 0, 0, time.FixedZone("CEST", 2*3600))
	updated := time.Date(2026, 10, 12, 15, 0, 0, 0, time.FixedZone("CEST", 2*3600))

	got, err := New(GooglePlace{
		PlaceID:                  "  ChIJN1t_tDeuEmsRUsoyG83frY4  ",
		InternationalPhoneNumber: " +33 1 23 45 67 89 ",
		Rating:                   &rating,
		UserRatingCount:          &count,
		BusinessStatus:           "  OPERATIONAL  ",
		WebsiteURL:               " https://example.com ",
		GoogleMapDirectionsURL:   " https://maps.google.com/dir ",
		GoogleMapURL:             " https://maps.google.com/place ",
		CreatedAt:                created,
		UpdatedAt:                updated,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if got.PlaceID != "ChIJN1t_tDeuEmsRUsoyG83frY4" {
		t.Fatalf("PlaceID = %q", got.PlaceID)
	}
	if got.ID == uuid.Nil || got.ID.Version() != 7 {
		t.Fatalf("ID = %v, want UUID v7", got.ID)
	}
	if got.InternationalPhoneNumber != "+33 1 23 45 67 89" {
		t.Fatalf("InternationalPhoneNumber = %q", got.InternationalPhoneNumber)
	}
	if got.Rating == nil || *got.Rating != 4.5 {
		t.Fatalf("Rating = %v", got.Rating)
	}
	if got.UserRatingCount == nil || *got.UserRatingCount != 42 {
		t.Fatalf("UserRatingCount = %v", got.UserRatingCount)
	}
	if got.BusinessStatus != BusinessStatusOperational {
		t.Fatalf("BusinessStatus = %q", got.BusinessStatus)
	}
	if !got.CreatedAt.Equal(created.UTC()) {
		t.Fatalf("CreatedAt = %v, want %v", got.CreatedAt, created.UTC())
	}
	if !got.UpdatedAt.Equal(updated.UTC()) {
		t.Fatalf("UpdatedAt = %v, want %v", got.UpdatedAt, updated.UTC())
	}
}

func TestNewAllowsMissingOptionalFields(t *testing.T) {
	got, err := New(GooglePlace{PlaceID: "ChIJN1t_tDeuEmsRUsoyG83frY4"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got.Rating != nil || got.UserRatingCount != nil {
		t.Fatalf("expected nil optional numerics, got rating=%v count=%v", got.Rating, got.UserRatingCount)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("timestamps must be set, got created=%v updated=%v", got.CreatedAt, got.UpdatedAt)
	}
}
