package write

import "testing"

func TestEuroAmountValue(t *testing.T) {
	cases := []struct {
		amount int
		want   string
	}{
		{amount: 2334, want: "2.334"},
		{amount: 1000, want: "1.000"},
		{amount: 0, want: "0.000"},
	}
	for _, tc := range cases {
		got, err := euroAmount(tc.amount).Value()
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Fatalf("amount %d = %v, want %s", tc.amount, got, tc.want)
		}
	}
}

func TestEuroAmountScan(t *testing.T) {
	var amount euroAmount
	if err := amount.Scan("2.399"); err != nil {
		t.Fatal(err)
	}
	if amount != 2399 {
		t.Fatalf("got %d", amount)
	}
	if err := amount.Scan("2.39900"); err != nil {
		t.Fatal(err)
	}
	if amount != 2399 {
		t.Fatalf("padded = %d", amount)
	}
	if err := amount.Scan("2.3991"); err == nil {
		t.Fatal("expected extra precision to fail")
	}
}
