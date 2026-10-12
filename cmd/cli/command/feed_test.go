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
		Published: 2,
		Unchanged: 8,
	}, time.Second)

	text := out.String()
	for _, want := range []string{
		"stations: 10",
		"published: 2",
		"unchanged: 8",
		"elapsed: 1s",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
	}
}
