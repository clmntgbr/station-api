package station

import "math"

// Location is a WGS84 point in decimal degrees.
type Location struct {
	Latitude  float64
	Longitude float64
}

// Valid reports whether the point is a real position.
// The null island (0, 0) is rejected: the feed uses it when coordinates are missing.
func (l Location) Valid() bool {
	if math.IsNaN(l.Latitude) || math.IsNaN(l.Longitude) || math.IsInf(l.Latitude, 0) || math.IsInf(l.Longitude, 0) {
		return false
	}
	if l.Latitude == 0 && l.Longitude == 0 {
		return false
	}
	return l.Latitude >= -90 && l.Latitude <= 90 && l.Longitude >= -180 && l.Longitude <= 180
}
