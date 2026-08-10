// A token sale simulation, built phase by phase. Phases 1 to 4 are done here.
// The price is a flat 1:1 for now. See README.md for the bonding curve.
package main

import (
	"errors"
	"fmt"
)

// Phase 1: variables and constants

const ( // phase-1
	// One whole token = 10^Decimals base units. pump.fun tokens use 6 decimals.
	Decimals = 6                // phase-1
	Unit     = int64(1_000_000) // phase-1

	// Base units of the base asset needed for one base unit of token.
	// 1 = a flat 1:1 sale. Const, not var: it's a rule of the sale, not state,
	// so nothing can reassign it by accident from the other side of the file.
	PriceBaseUnits = int64(1) // phase-1
) // phase-1

// var, not const: these are things you'd want to change at runtime later
// (Phase 7 turns them into CLI flags), and startingBalance is computed from
// Unit rather than typed out as a literal.
// var and not := because := only works inside a function.
var ( // phase-1
	numUsers        = 5          // phase-1
	startingBalance = 100 * Unit // phase-1
) // phase-1

// Phase 2: the User struct

// User is one participant in the sale.
//
// Balances are int64 counts of base units, not float64. float64 can't hold 0.1
// exactly, so balances drift apart after enough arithmetic and stop summing to
// the supply. Chains store integers (a lamport is exactly this) and only divide
// when printing. formatUnits does the dividing.
//
// Fields are capitalized, which means exported. Doesn't matter in a one package
// program, but it's the right default: move this into its own package and
// unexported fields become unreachable. It's also what lets %+v see them.
type User struct { // phase-2
	Name         string // phase-2
	BaseBalance  int64  // base asset held, in base units // phase-2
	TokenBalance int64  // sale token held, in base units // phase-2
} // phase-2

// Phase 3: methods, functions, and errors

// Sentinel errors, so callers can check with errors.Is instead of matching on
// the message text.
var ( // phase-3
	ErrInvalidAmount     = errors.New("amount must be positive") // phase-3
	ErrInsufficientFunds = errors.New("insufficient balance")    // phase-3
) // phase-3

// Fund adds to the user's base asset balance.
// Pointer receiver because it has to change the real user, not a copy.
func (u *User) Fund(amount int64) error { // phase-3
	if amount <= 0 { // phase-3
		return fmt.Errorf("fund %s: %w", formatUnits(amount), ErrInvalidAmount) // phase-3
	} // phase-3
	u.BaseBalance += amount // phase-3
	return nil              // phase-3
} // phase-3

// The same thing with a VALUE receiver, kept as the Phase 3 experiment. It
// compiles, it runs, and it does nothing.
//
// Calling a value receiver method copies the whole struct first. The u in here
// is a different User holding the same numbers; the += lands on that copy and
// the copy is thrown away on return. Only *User gets you the original.
//
// Lowercase name because it's a demo, not something to actually call.
func (u User) fundByValue(amount int64) { // phase-3
	u.BaseBalance += amount // phase-3
} // phase-3

// Buy purchases tokenAmount base units at the current price.
//
// Returns an error instead of printing one, because only the caller knows what
// a failure means. Phase 5 skips that user and keeps going, a test asserts on
// it, a CLI exits non zero. Printing decides for all of them, and you can't ask
// a fmt.Println afterwards whether it fired.
func (u *User) Buy(tokenAmount int64) error { // phase-3
	if tokenAmount <= 0 { // phase-3
		return fmt.Errorf("buy %s: %w", formatUnits(tokenAmount), ErrInvalidAmount) // phase-3
	} // phase-3

	// Can't overflow while the price is 1. A bonding curve would need a check
	// here, since costs near the top of the curve get very large.
	cost := tokenAmount * PriceBaseUnits // phase-3

	if cost > u.BaseBalance { // phase-3
		return fmt.Errorf("buy %s tokens: costs %s, balance is %s: %w", // phase-3
			formatUnits(tokenAmount), formatUnits(cost), formatUnits(u.BaseBalance),
			ErrInsufficientFunds)
	} // phase-3

	// Both checks passed before anything changed, so a rejected buy leaves the
	// user untouched. Deducting first is how you end up with negative balances.
	u.BaseBalance -= cost         // phase-3
	u.TokenBalance += tokenAmount // phase-3
	return nil                    // phase-3
} // phase-3

// printSummary prints one line for a user.
//
// A plain function rather than a method, taking User by value: it only reads,
// and how a user gets displayed isn't really the type's business. Keeping it
// out here means the same User can be a table row now and JSON later.
func printSummary(u User) { // phase-3
	fmt.Printf("  %-8s  base: %12s   tokens: %12s\n", // phase-3
		u.Name, formatUnits(u.BaseBalance), formatUnits(u.TokenBalance))
} // phase-3

// formatUnits turns base units into something readable. The only place in the
// program where a decimal point exists. The arithmetic never sees a fraction.
func formatUnits(v int64) string { // phase-3
	sign := "" // phase-3
	if v < 0 { // phase-3
		sign = "-" // phase-3
		v = -v     // phase-3
	} // phase-3
	// %0*d reads its width from the Decimals operand, so the zero padding can't
	// fall out of sync with the constant.
	return fmt.Sprintf("%s%d.%0*d", sign, v/Unit, Decimals, v%Unit) // phase-3
} // phase-3

// Phase 4: slices and loops

// newUsers builds n users, growing the slice with append.
//
// make([]User, 0, n) is empty but with room for n already reserved, so append
// never has to reallocate and copy. The other two options:
//   - nil slice (var users []User). append works, but it grows and copies as it
//     goes. Right when you don't know n, wasteful when you do.
//   - slice literal []User{...}, for a fixed set you write out by hand.
func newUsers(n int, startingBalance int64) []User { // phase-4
	if n < 0 { // phase-4
		n = 0 // make panics on negative capacity // phase-4
	} // phase-4

	users := make([]User, 0, n) // phase-4
	for i := range n {          // phase-4
		users = append(users, User{ // phase-4
			Name:        fmt.Sprintf("user%02d", i+1), // phase-4
			BaseBalance: startingBalance,              // phase-4
			// TokenBalance left out. Omitted fields get their zero value,
			// which for int64 is the 0 we want anyway.
		}) // phase-4
	} // phase-4
	return users // phase-4
} // phase-4

// Same slice, built the other way: allocate all n up front, assign by index.
//
// make([]User, n) hands back n zero valued Users that already exist, so this
// assigns instead of appending. The trap is mixing the two. make([]User, n)
// followed by append gives you n empty users and then your real ones.
//
// Both versions are fine. Indexing when you know n, append when users show up
// conditionally or one at a time.
func newUsersPresized(n int, startingBalance int64) []User { // phase-4
	if n < 0 { // phase-4
		n = 0 // phase-4
	} // phase-4

	users := make([]User, n) // phase-4
	for i := range users {   // phase-4
		users[i] = User{ // phase-4
			Name:        fmt.Sprintf("user%02d", i+1), // phase-4
			BaseBalance: startingBalance,              // phase-4
		} // phase-4
	} // phase-4
	return users // phase-4
} // phase-4

// main walks the checkpoints in order.
func main() {
	fmt.Printf("Token sale: price %s base units per token base unit, %d decimals\n",
		formatUnits(PriceBaseUnits*Unit), Decimals)

	// Phase 2 checkpoint: one user built by hand, printed with %+v.
	fmt.Println("\n== Phase 2: one user, struct literal ==") // phase-2
	alice := User{                                           // phase-2
		Name:         "alice",   // phase-2
		BaseBalance:  50 * Unit, // phase-2
		TokenBalance: 0,         // phase-2
	} // phase-2
	fmt.Printf("  %+v\n", alice)                                        // phase-2
	fmt.Println("  (raw base units above, printSummary formats them:)") // phase-2
	printSummary(alice)                                                 // phase-2

	// Phase 3 checkpoint, part one: watch the value receiver do nothing.
	fmt.Println("\n== Phase 3a: the value receiver trap ==")                   // phase-3
	fmt.Printf("  before fundByValue: %s\n", formatUnits(alice.BaseBalance))   // phase-3
	alice.fundByValue(25 * Unit)                                               // phase-3
	fmt.Printf("  after  fundByValue: %s   <- unchanged, it mutated a copy\n", // phase-3
		formatUnits(alice.BaseBalance))

	// Part two: the pointer version, and both branches of Buy.
	fmt.Println("\n== Phase 3b: pointer receivers actually mutate ==") // phase-3
	if err := alice.Fund(25 * Unit); err != nil {                      // phase-3
		fmt.Printf("  fund failed: %v\n", err) // phase-3
	} else { // phase-3
		fmt.Printf("  after  Fund:        %s\n", formatUnits(alice.BaseBalance)) // phase-3
	} // phase-3

	// A buy she can afford.
	if err := alice.Buy(30 * Unit); err != nil { // phase-3
		fmt.Printf("  buy failed: %v\n", err) // phase-3
	} else { // phase-3
		fmt.Println("  bought 30 tokens") // phase-3
		printSummary(alice)               // phase-3
	} // phase-3

	// A buy she can't, so the error path, plus proof nothing moved.
	if err := alice.Buy(1000 * Unit); err != nil { // phase-3
		fmt.Printf("  buy rejected: %v\n", err)                     // phase-3
		fmt.Printf("  errors.Is(err, ErrInsufficientFunds) = %t\n", // phase-3
			errors.Is(err, ErrInsufficientFunds))
		fmt.Println("  balances after the failed buy (must be untouched):") // phase-3
		printSummary(alice)                                                 // phase-3
	} // phase-3

	// Phase 4 checkpoint: N users in a slice, printed in a loop.
	// range copies each element into u, which is safe here because
	// printSummary only reads. Phase 5 is where that stops being true.
	fmt.Printf("\n== Phase 4: %d users, built with append ==\n", numUsers) // phase-4
	users := newUsers(numUsers, startingBalance)                           // phase-4
	for _, u := range users {                                              // phase-4
		printSummary(u) // phase-4
	} // phase-4

	fmt.Printf("\n== Phase 4: the same %d users, pre sized and indexed ==\n", numUsers) // phase-4
	sameUsers := newUsersPresized(numUsers, startingBalance)                            // phase-4
	for _, u := range sameUsers {                                                       // phase-4
		printSummary(u) // phase-4
	} // phase-4

	fmt.Println("\nPhases 1 to 4 complete. Phase 5 (everyone buys) is next.")
}
