package main

import (
	"errors" // errors.Is — checking the sentinel, not the message text, same rule Buy itself follows
	"testing"
)

// TestUserBuy is table-driven: one slice of cases, one loop, one assertion
// block — adding a case means adding a row, not writing a new test
// function. t.Run gives each row its own named subtest, so `go test -run
// TestUserBuy/insufficient` can target one row directly, and a failure
// reports which row failed instead of just "TestUserBuy failed."
func TestUserBuy(t *testing.T) { // phase-7
	tests := []struct { // phase-7
		name           string // phase-7: subtest name, shows up in `go test -v` and -run
		startBalance   int64  // phase-7
		buyAmount      int64  // phase-7
		wantErr        error  // phase-7: nil means "must succeed"
		wantBaseAfter  int64  // phase-7
		wantTokenAfter int64  // phase-7
	}{
		{
			name:           "affordable buy succeeds",
			startBalance:   100 * Unit,
			buyAmount:      30 * Unit,
			wantErr:        nil,
			wantBaseAfter:  70 * Unit,
			wantTokenAfter: 30 * Unit,
		},
		{
			// The case the brief explicitly asked for. Checking
			// wantBaseAfter/wantTokenAfter here isn't redundant with
			// wantErr — it's the actual proof that Buy validated before
			// mutating (main.go:104-105), not just that it returned an
			// error.
			name:           "insufficient balance is rejected and leaves balances untouched",
			startBalance:   10 * Unit,
			buyAmount:      30 * Unit,
			wantErr:        ErrInsufficientFunds,
			wantBaseAfter:  10 * Unit,
			wantTokenAfter: 0,
		},
		{
			name:           "zero amount is rejected",
			startBalance:   100 * Unit,
			buyAmount:      0,
			wantErr:        ErrInvalidAmount,
			wantBaseAfter:  100 * Unit,
			wantTokenAfter: 0,
		},
		{
			name:           "negative amount is rejected",
			startBalance:   100 * Unit,
			buyAmount:      -5 * Unit,
			wantErr:        ErrInvalidAmount,
			wantBaseAfter:  100 * Unit,
			wantTokenAfter: 0,
		},
	}

	for _, tt := range tests { // phase-7: range-by-value is fine here — tt is only read, never mutated, same rule Phase 4's print loops relied on
		t.Run(tt.name, func(t *testing.T) { // phase-7: closes over tt directly — safe under Go 1.22+ (go.mod: go 1.26.5) per-iteration loop variables, same fact noted in main.go's Phase 7c goroutines
			u := User{Name: "test", BaseBalance: tt.startBalance} // phase-7: fresh User per subtest, never shared across table rows
			err := u.Buy(tt.buyAmount)                            // phase-7: *User method on an addressable local — real mutation, not a copy

			switch {
			case tt.wantErr == nil && err != nil: // phase-7
				t.Fatalf("Buy(%s) = %v, want nil", formatUnits(tt.buyAmount), err) // phase-7
			case tt.wantErr != nil && !errors.Is(err, tt.wantErr): // phase-7
				t.Fatalf("Buy(%s) = %v, want error wrapping %v", formatUnits(tt.buyAmount), err, tt.wantErr) // phase-7
			} // phase-7

			if u.BaseBalance != tt.wantBaseAfter { // phase-7
				t.Errorf("BaseBalance = %s, want %s", formatUnits(u.BaseBalance), formatUnits(tt.wantBaseAfter)) // phase-7
			} // phase-7
			if u.TokenBalance != tt.wantTokenAfter { // phase-7
				t.Errorf("TokenBalance = %s, want %s", formatUnits(u.TokenBalance), formatUnits(tt.wantTokenAfter)) // phase-7
			} // phase-7
		})
	}
}

// TestSaleBuy covers the newer, curve-priced path — smaller than
// TestUserBuy because the pricing math itself isn't re-verified here
// (that's what hand-tracing curveCost's formula is for); this only checks
// that Sale.Buy's validation and error wrapping behave the same way
// User.Buy's do.
func TestSaleBuy(t *testing.T) { // phase-7
	tests := []struct { // phase-7
		name         string
		startBalance int64
		buyAmount    int64
		wantErr      error
	}{
		{"affordable buy succeeds", 100 * Unit, 30 * Unit, nil},
		{"insufficient balance is rejected", 1 * Unit, 30 * Unit, ErrInsufficientFunds},
		{"zero amount is rejected", 100 * Unit, 0, ErrInvalidAmount},
	}

	for _, tt := range tests { // phase-7
		t.Run(tt.name, func(t *testing.T) { // phase-7
			var sale Sale                                         // phase-7: zero value, one fresh Sale per subtest — no state leaks between rows
			u := User{Name: "test", BaseBalance: tt.startBalance} // phase-7
			err := sale.Buy(&u, tt.buyAmount)                     // phase-7: &u — u is addressable, same rule as everywhere else Sale.Buy is called

			switch {
			case tt.wantErr == nil && err != nil: // phase-7
				t.Fatalf("Buy(%s) = %v, want nil", formatUnits(tt.buyAmount), err) // phase-7
			case tt.wantErr != nil && !errors.Is(err, tt.wantErr): // phase-7
				t.Fatalf("Buy(%s) = %v, want error wrapping %v", formatUnits(tt.buyAmount), err, tt.wantErr) // phase-7
			} // phase-7
		})
	}
}
