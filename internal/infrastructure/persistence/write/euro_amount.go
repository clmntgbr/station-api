package write

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
)

// euroAmount stores thousandths of a euro as numeric(6,3) euros.
// 2334 becomes 2.334.
type euroAmount int

func (a euroAmount) Value() (driver.Value, error) {
	amount := int(a)
	if amount < 0 {
		return nil, fmt.Errorf("negative price %d", amount)
	}
	return fmt.Sprintf("%d.%03d", amount/1000, amount%1000), nil
}

func (a *euroAmount) Scan(value any) error {
	if value == nil {
		return fmt.Errorf("null price")
	}
	var raw string
	switch v := value.(type) {
	case string:
		raw = v
	case []byte:
		raw = string(v)
	default:
		return fmt.Errorf("unsupported price type %T", value)
	}
	amount, err := parseEuroAmount(raw)
	if err != nil {
		return err
	}
	*a = euroAmount(amount)
	return nil
}

func parseEuroAmount(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("empty price")
	}
	negative := strings.HasPrefix(raw, "-")
	if negative {
		raw = strings.TrimPrefix(raw, "-")
	}
	wholeText, fracText, _ := strings.Cut(raw, ".")
	if wholeText == "" {
		wholeText = "0"
	}
	whole, err := strconv.Atoi(wholeText)
	if err != nil {
		return 0, fmt.Errorf("parse price %q: %w", raw, err)
	}
	for _, c := range fracText {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("parse price fraction %q", fracText)
		}
	}
	if len(fracText) > 3 {
		for _, c := range fracText[3:] {
			if c != '0' {
				return 0, fmt.Errorf("price %q has more than 3 decimal places", raw)
			}
		}
		fracText = fracText[:3]
	}
	for len(fracText) < 3 {
		fracText += "0"
	}
	frac, err := strconv.Atoi(fracText)
	if err != nil {
		return 0, err
	}
	amount := whole*1000 + frac
	if negative {
		amount = -amount
	}
	return amount, nil
}
