package command

import (
	"bytes"
	"strings"
	"testing"
	"time"

	stationcmd "go-api/internal/application/command/station"
)

func TestWriteFeedSummary(t *testing.T) {
	var out bytes.Buffer
	writeFeedSummary(&out, stationcmd.Result{
		Stations:  10,
		Created:   1,
		Info:      2,
		Prices:    3,
		Unchanged: 4,
	}, time.Second)

	text := out.String()
	for _, want := range []string{
		"stations: 10",
		"created: 1",
		"info: 2",
		"prices: 3",
		"unchanged: 4",
		"elapsed: 1s",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
	}
}
