package station

// Hours is the weekly opening schedule of a station.
type Hours struct {
	// Automate24 is true when the pump is available 24 hours a day.
	Automate24 bool
	Days       []Day
}

// Day is one weekday. ID 1 is Monday and ID 7 is Sunday.
// A closed day can still carry a slot: automated pumps publish 01:00–01:00.
type Day struct {
	ID     int
	Name   string
	Closed bool
	Slots  []Slot
}

// Slot is one opening interval on a day.
type Slot struct {
	Opens  Clock
	Closes Clock
}

// Clock is a time of day in 24-hour form.
type Clock struct {
	Hour   int
	Minute int
}

// Valid reports whether the clock is a real time of day.
func (c Clock) Valid() bool {
	return c.Hour >= 0 && c.Hour <= 23 && c.Minute >= 0 && c.Minute <= 59
}
