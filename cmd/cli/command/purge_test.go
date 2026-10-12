package command

import "testing"

func TestTablesToPurgeKeepsUsersAndMigrations(t *testing.T) {
	got := tablesToPurge([]string{
		"current_prices",
		"prices_history",
		"goose_db_version",
		"outbox_events",
		"processed_events",
		"stations",
		"users",
	})
	want := []string{
		"current_prices",
		"prices_history",
		"outbox_events",
		"processed_events",
		"stations",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestQuoteTableRejectsUnsafeNames(t *testing.T) {
	quoted, err := quoteTable("current_prices")
	if err != nil || quoted != `"current_prices"` {
		t.Fatalf("quoted = %s, err = %v", quoted, err)
	}
	if _, err := quoteTable(`stations"; DROP TABLE users`); err == nil {
		t.Fatal("expected unsafe name to be rejected")
	}
}
