package station

import "go-api/internal/domain/price"

// PriceChange is one fuel whose amount differs from the stored price.
// PreviousAmount is nil when the station did not sell that fuel yet.
type PriceChange struct {
	Price          price.Price
	PreviousAmount *int
}

// DiffPrices returns the fuels whose amount changed, in incoming order.
// A fuel that keeps the same amount is unchanged, even if its source time moved.
// Fuels absent from incoming are not listed: the caller drops them from current prices.
func DiffPrices(current, incoming []price.Price) []PriceChange {
	stored := make(map[price.Fuel]price.Price, len(current))
	for _, item := range current {
		stored[item.Fuel] = item
	}

	changes := make([]PriceChange, 0)
	for _, next := range incoming {
		previous, ok := stored[next.Fuel]
		if !ok {
			changes = append(changes, PriceChange{Price: next})
			continue
		}
		if previous.Amount == next.Amount {
			continue
		}
		amount := previous.Amount
		changes = append(changes, PriceChange{Price: next, PreviousAmount: &amount})
	}
	return changes
}
