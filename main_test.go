package main

import (
	"crypto/ed25519" // for signing a request in the test itself, playing the role of the buyer's wallet
	"errors"         // errors.Is — checking the sentinel, not the message text, same rule Buy itself follows
	"fmt"            // building receipt buyer names for TestReceiptChain
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

// TestSignedBuy covers both directions: a genuinely signed request goes
// through, and a forged one is rejected — the actual property SignedBuy
// adds on top of Sale.Buy.
func TestSignedBuy(t *testing.T) { // phase-8
	u, priv, err := newSignedUser("test", 100*Unit) // phase-8
	if err != nil {                                 // phase-8
		t.Fatalf("newSignedUser: %v", err) // phase-8
	} // phase-8

	var sale Sale                          // phase-8
	msg := signBuyMessage(u.Name, 30*Unit) // phase-8
	validSig := ed25519.Sign(priv, msg)    // phase-8: signing here, with priv, the way a wallet would — never inside SignedBuy itself

	if err := sale.SignedBuy(u, 30*Unit, validSig); err != nil { // phase-8
		t.Fatalf("SignedBuy with valid signature = %v, want nil", err) // phase-8
	} // phase-8

	forged := make([]byte, len(validSig)) // phase-8
	copy(forged, validSig)                // phase-8
	forged[0] ^= 0xFF                     // phase-8: flip one bit — enough to invalidate an ed25519 signature completely
	// Same u, same sale, already-successful buyer — proof forgery still
	// doesn't work even against a real, funded account.
	err = sale.SignedBuy(u, 30*Unit, forged)  // phase-8
	if !errors.Is(err, ErrInvalidSignature) { // phase-8
		t.Fatalf("SignedBuy with forged signature = %v, want ErrInvalidSignature", err) // phase-8
	} // phase-8
}

// TestReceiptChain covers both directions: an honestly built chain
// verifies clean, and tampering with any past entry is caught.
func TestReceiptChain(t *testing.T) { // phase-8
	var receipts []Receipt                             // phase-8
	amounts := []int64{10 * Unit, 20 * Unit, 5 * Unit} // phase-8
	var prevHash [32]byte                              // phase-8
	for i, amt := range amounts {                      // phase-8: range-by-value — only reading amt, never mutating
		r := newReceipt(prevHash, fmt.Sprintf("user%02d", i+1), amt, int64(i+1)*amt) // phase-8
		receipts = append(receipts, r)                                               // phase-8
		prevHash = r.Hash                                                            // phase-8
	} // phase-8

	if broken := verifyChain(receipts); broken != -1 { // phase-8
		t.Fatalf("verifyChain on an untampered chain = %d, want -1", broken) // phase-8
	} // phase-8

	receipts[0].TokenAmount = 999 * Unit              // phase-8: tamper without recomputing Hash — exactly what an attacker would try
	if broken := verifyChain(receipts); broken != 0 { // phase-8
		t.Fatalf("verifyChain after tampering with receipt 0 = %d, want 0", broken) // phase-8
	} // phase-8
}
