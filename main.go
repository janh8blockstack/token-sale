// A token sale simulation, built phase by phase. Phases 1 to 4 are done here.
// The price is a flat 1:1 for now. See README.md for the bonding curve.
package main

import (
	"crypto/ed25519" // phase-8: real signing — the same algorithm Solana uses for every transaction
	"crypto/rand"    // phase-8: a real secret (a private key) — the one place in this file crypto/rand is the correct tool, unlike simulation randomness elsewhere
	"crypto/sha256"  // phase-8: the receipt chain's hash
	"errors"         // phase-3
	"flag"           // phase-7: CLI flags for numUsers, startingBalance, and the curve
	"fmt"            // phase-1
	"math"           // phase-7: math.Exp for the curve, math.Ceil for round-up-on-cost
	"sync"           // phase-7: sync.Mutex guards Sale's shared state
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
	// PublicKey added in Phase 8, not Phase 2 — safe because every User{}
	// literal in this file already names its fields, so an omitted
	// PublicKey just takes its zero value (nil), same as TokenBalance did
	// back in Phase 4 (main.go:151-154). Phases 1-7's users simply have no
	// registered key; only Phase 8's newSignedUser sets one.
	PublicKey ed25519.PublicKey // phase-8: this User's address, not a secret — the matching private key never lives on this struct
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

// Phase 5: batch operations over a slice

// buyAll attempts to buy amount tokens for every user in users, in place,
// and reports one error per user instead of stopping at the first failure.
//
// Mechanism vs policy, the same split Buy itself already draws (main.go:87):
// buyAll only performs the batch and hands back what happened to each user.
// It doesn't print, doesn't skip, doesn't decide anything is fatal — that's
// main's job below. A batch operation that also decided how to react to its
// own failures couldn't be reused by a future CLI or test that wants to
// react differently.
//
// []User, not []*User: a slice's backing array is already shared memory, so
// users[i].Buy(...) inside the loop reaches the exact same User main holds —
// no pointer slice is needed just for that. (A slice of pointers earns its
// place once Phase 7's map needs identity that outlives its slot in a
// slice.)
func buyAll(users []User, amount int64) []error { // phase-5
	results := make([]error, len(users)) // phase-5: one result slot per user, index-aligned with users
	for i := range users {               // phase-5: index-only range — reaches users[i] directly, no copy in between
		results[i] = users[i].Buy(amount) // phase-5: pointer receiver on the real element — a success here really mutates it
	} // phase-5
	return results // phase-5
} // phase-5

// Phase 7: shared market state and the exponential pricing curve

// basePrice and curveK define the curve: price(supply) = basePrice ×
// e^(curveK × supply). basePrice matches Phase 1-6's flat PriceBaseUnits,
// so the curve starts at exactly the same price the flat phases used.
//
// curveK is expressed per BASE UNIT, not per whole token, because
// totalTokensSold below is tracked in base units — the same reason Unit
// exists at all (main.go:15). 0.001 "per whole token" scaled down by Unit
// keeps the exponent sane at realistic totalTokensSold values instead of
// exploding; using 0.001 directly against a base-unit count would blow the
// curve up almost immediately.
//
// var, not const, unlike PriceBaseUnits (main.go:18-19) — this is exactly
// the CLI-flags stretch goal the Phase 1 comment predicted (main.go:25-26).
// flag needs an addressable variable to write into, which a const can never
// be. The values below are only the defaults; main() may overwrite them via
// -price and -k before any checkpoint reads them.
var ( // phase-7
	basePrice = float64(PriceBaseUnits) // phase-7: default 1.0 — same starting price the flat phases used
	curveK    = 0.001 / float64(Unit)   // phase-7: default — 0.1% per whole token, scaled to base units
) // phase-7

// price returns the current price for the next base unit of token, given
// how many base units have sold so far. This is the project's own curve —
// exponential, not pump.fun's constant-product formula. See README.md
// "Where the curve goes" and "Exponential vs. hyperbolic, precisely" for
// why those are different shapes, not the same shape with different
// constants.
func price(totalTokensSold int64) float64 { // phase-7
	return basePrice * math.Exp(curveK*float64(totalTokensSold)) // phase-7
} // phase-7

// curveCost returns the total price to buy tokenAmount base units, starting
// from totalTokensSold already sold — the INTEGRAL of price over that
// range, not price(totalTokensSold)*tokenAmount. Price moves while an
// order fills; charging the spot price at the start would undercharge
// every buy bigger than an infinitesimal one.
//
// Named curveCost, not cost: User.Buy already has a local variable named
// cost (main.go:96) for a different, simpler meaning (tokenAmount times a
// flat price). Same word, two meanings, in the same file — reusing it here
// would shadow-not-conflict (Go allows it) but reads as a mistake on sight.
//
// e^x has no exact int64 form, so this computes in float64. The caller
// (Sale.Buy) rounds to int64 base units at the boundary, right before a
// balance is touched — the one deliberate exception to the int64-only
// money rule (main.go:36-39), same exception README.md documents.
func curveCost(totalTokensSold, tokenAmount int64) float64 { // phase-7
	sold := float64(totalTokensSold) // phase-7
	amount := float64(tokenAmount)   // phase-7
	// (basePrice/curveK) × (e^(k(sold+amount)) − e^(k·sold)) — the closed
	// form of ∫ price(s) ds from sold to sold+amount.
	return (basePrice / curveK) * (math.Exp(curveK*(sold+amount)) - math.Exp(curveK*sold)) // phase-7
} // phase-7

// Sale holds state shared by every buyer — the thing Phases 1-6 never
// needed, since price used to be a fixed constant nobody had to share. A
// curve isn't a price, it's market state: how many tokens have sold so
// far, which every buyer's price now depends on.
//
// Fields unexported, unlike User's: nothing outside this file constructs a
// Sale from a literal or needs %+v on it (main.go:41-43 is why User's are
// exported; that reasoning doesn't apply here).
type Sale struct { // phase-7
	totalTokensSold int64      // phase-7: base units sold so far, across every buyer
	mu              sync.Mutex // phase-7: guards totalTokensSold and the balance mutation together, see Buy
} // phase-7

// Buy purchases tokenAmount base units of token for u, priced off the
// curve's current state instead of a fixed constant. This is why Buy moved
// from *User to *Sale (README.md: "the price stops being a property of the
// buyer and becomes a property of the market") — pricing needs
// totalTokensSold, and only Sale has one.
//
// Pointer receiver on *Sale for the same reason *User's methods needed one
// (main.go:59-60): Lock, the balance mutation, and totalTokensSold's
// update all have to land on the real shared Sale, not a copy.
func (s *Sale) Buy(u *User, tokenAmount int64) error { // phase-7
	if tokenAmount <= 0 { // phase-7
		return fmt.Errorf("buy %s: %w", formatUnits(tokenAmount), ErrInvalidAmount) // phase-7
	} // phase-7

	// Locked from here through the mutation at the bottom: totalTokensSold
	// is read to price this buy, then written to record it, and no other
	// goroutine's Buy can interleave between those two steps while the
	// lock is held. Phase 5's buyAll got this for free from being a plain
	// sequential loop; this is what recreates the same guarantee once
	// callers can be concurrent goroutines instead (the concurrency
	// stretch goal, still to come).
	s.mu.Lock()         // phase-7
	defer s.mu.Unlock() // phase-7

	rawCost := curveCost(s.totalTokensSold, tokenAmount) // phase-7
	// Round up: the protocol's favour, never the buyer's — the rounding
	// direction rule the flat-price phases never needed but the curve does
	// (README.md "Design decisions worth knowing").
	costInBaseUnits := int64(math.Ceil(rawCost)) // phase-7

	if costInBaseUnits > u.BaseBalance { // phase-7
		return fmt.Errorf("buy %s tokens: costs %s, balance is %s: %w", // phase-7
			formatUnits(tokenAmount), formatUnits(costInBaseUnits), formatUnits(u.BaseBalance),
			ErrInsufficientFunds)
	} // phase-7

	// Same validate-before-mutate discipline as User.Buy (main.go:102-104):
	// a rejected buy leaves both u and s untouched, byte for byte.
	u.BaseBalance -= costInBaseUnits // phase-7
	u.TokenBalance += tokenAmount    // phase-7
	s.totalTokensSold += tokenAmount // phase-7
	return nil                       // phase-7
} // phase-7

// unsafeBuy is Sale.Buy's exact logic with the lock deliberately removed —
// the Phase 7 sibling of fundByValue (Phase 3a) and the range-copy loop
// (Phase 5a): broken on purpose, kept only to demonstrate the failure mode
// live before the fix. Never called outside the Phase 7b checkpoint.
func (s *Sale) unsafeBuy(u *User, tokenAmount int64) error { // phase-7
	if tokenAmount <= 0 { // phase-7
		return fmt.Errorf("buy %s: %w", formatUnits(tokenAmount), ErrInvalidAmount) // phase-7
	} // phase-7

	// No s.mu.Lock() here. That's the entire point.
	rawCost := curveCost(s.totalTokensSold, tokenAmount) // phase-7
	costInBaseUnits := int64(math.Ceil(rawCost))         // phase-7

	if costInBaseUnits > u.BaseBalance { // phase-7
		return fmt.Errorf("buy %s tokens: costs %s, balance is %s: %w", // phase-7
			formatUnits(tokenAmount), formatUnits(costInBaseUnits), formatUnits(u.BaseBalance),
			ErrInsufficientFunds)
	} // phase-7

	u.BaseBalance -= costInBaseUnits // phase-7: safe — each goroutine touches a different u, no shared memory here
	u.TokenBalance += tokenAmount    // phase-7: same — private to this u
	// The race: read s.totalTokensSold, add to it, write it back — three
	// separate machine steps, not one atomic operation. Two goroutines can
	// both read the same value here before either writes, and one
	// goroutine's update silently overwrites the other's. This is the only
	// line in the function actually touching memory shared across
	// goroutines, and it's exactly the line missing Sale.Buy's lock.
	s.totalTokensSold += tokenAmount // phase-7
	return nil                       // phase-7
} // phase-7

// Phase 7: maps

// newUserMap builds n users keyed by name, holding pointers.
//
// map[string]*User, not map[string]User: a map value isn't addressable in
// Go — there's no &m["key"], and no way to call a pointer-receiver method
// through an indexed map value directly, the way users[i] works for a
// slice. Storing *User sidesteps that entirely: looking a user up by name
// always hands back the one real User, ready for Sale.Buy's pointer
// receiver, with no copy anywhere in the path.
func newUserMap(n int, startingBalance int64) map[string]*User { // phase-7
	users := make(map[string]*User, n) // phase-7: n here is a capacity hint, not a length — maps have no make([]T, n)-style "n zero elements" form
	for i := range n {                 // phase-7
		name := fmt.Sprintf("user%02d", i+1) // phase-7
		users[name] = &User{                 // phase-7: &User{...} takes the address of a freshly allocated User — escapes to the heap, outlives this function call
			Name:        name,            // phase-7
			BaseBalance: startingBalance, // phase-7
		} // phase-7
	} // phase-7
	return users // phase-7
} // phase-7

// Phase 7: interfaces

// Buyer is the smallest interface that captures "can attempt a purchase."
// Only *User satisfies it here — Sale.Buy takes an extra *User parameter, a
// different method signature, so Sale does NOT satisfy Buyer. That's not an
// oversight: main.go:270-274 already makes this point in prose, and this is
// the same fact showing up in the type system — pricing stopped being
// something a buyer carries around, so Sale's Buy couldn't keep User.Buy's
// shape even if it wanted to.
//
// Doesn't buy much with exactly one implementer — the payoff shows up once
// something needs to accept "anything buyable" without caring which
// concrete type it is. Left small on purpose, per the brief.
type Buyer interface { // phase-7
	Buy(tokenAmount int64) error // phase-7
} // phase-7

// attemptPurchase takes a Buyer, not specifically a *User — proof the
// interface does real work rather than just decorating the code. Any future
// type with a matching Buy(int64) error method satisfies Buyer automatically
// (Go interfaces are implicit, no "implements" declaration needed), and this
// function wouldn't need to change to accept it.
func attemptPurchase(b Buyer, tokenAmount int64) error { // phase-7
	return b.Buy(tokenAmount) // phase-7
} // phase-7

// Phase 8: real identity via crypto/ed25519

// ErrInvalidSignature covers both "no key registered" and "signature
// doesn't verify" — one sentinel, not two, since a caller checking
// errors.Is(err, ErrInvalidSignature) shouldn't need to care which of
// those happened; either way, the request isn't authenticated.
var ErrInvalidSignature = errors.New("invalid signature") // phase-8

// newSignedUser builds one User with a fresh ed25519 keypair, and returns
// the private key SEPARATELY — never stored on User at all. Same reason a
// real wallet's private key never touches a server: only the public key
// (the address) is something the User/Sale side ever needs to know. The
// caller of newSignedUser is the only one who ever holds the private key,
// and only for as long as it takes to sign a request.
func newSignedUser(name string, startingBalance int64) (*User, ed25519.PrivateKey, error) { // phase-8
	pub, priv, err := ed25519.GenerateKey(rand.Reader) // phase-8: crypto/rand.Reader — this is a genuine secret, the one case in this file where crypto/rand (not math/rand/v2) is the right tool
	if err != nil {                                    // phase-8
		return nil, nil, fmt.Errorf("generate keypair for %s: %w", name, err) // phase-8
	} // phase-8
	u := &User{ // phase-8
		Name:        name,            // phase-8
		BaseBalance: startingBalance, // phase-8
		PublicKey:   pub,             // phase-8
	} // phase-8
	return u, priv, nil // phase-8
} // phase-8

// signBuyMessage returns the exact bytes a buyer signs to authorize one
// purchase — canonical and unambiguous, so a signature over "buy 30
// tokens" can't be reinterpreted as authorizing 3000. No nonce here, which
// a real system would need (the same valid signature could otherwise be
// replayed to buy twice) — left out to keep this checkpoint focused on
// verification itself, not a full anti-replay design.
func signBuyMessage(userName string, tokenAmount int64) []byte { // phase-8
	return fmt.Appendf(nil, "buy:%s:%d", userName, tokenAmount) // phase-8
} // phase-8

// SignedBuy is Sale.Buy with one gate in front: the caller must prove they
// hold the private key matching u.PublicKey by supplying a valid signature
// over signBuyMessage(u.Name, tokenAmount). This is authenticity — a
// different property from Sale.Buy's mutex, which is about ordering. A
// forged request can't get through here even under concurrent access, and
// a genuinely signed request can still race another one the exact way
// Phase 7c demonstrated — the two mechanisms solve different problems, and
// neither substitutes for the other.
func (s *Sale) SignedBuy(u *User, tokenAmount int64, signature []byte) error { // phase-8
	if len(u.PublicKey) == 0 { // phase-8
		return fmt.Errorf("signed buy for %s: %w", u.Name, ErrInvalidSignature) // phase-8: no key registered — reuses ErrInvalidSignature rather than inventing a third failure mode
	} // phase-8
	message := signBuyMessage(u.Name, tokenAmount)        // phase-8
	if !ed25519.Verify(u.PublicKey, message, signature) { // phase-8: constant-time under the hood, not a plain byte == on the signature
		return fmt.Errorf("signed buy for %s: %w", u.Name, ErrInvalidSignature) // phase-8
	} // phase-8
	// Verified — hand off to the real Buy rather than duplicating its
	// pricing/mutation logic. One source of truth; this method only adds
	// the gate in front of it.
	return s.Buy(u, tokenAmount) // phase-8
} // phase-8

// Phase 8: a hash-chained receipt log

// Receipt is one link in the chain — a record of one successful buy, bound
// to the record before it via PrevHash. Changing any past receipt changes
// its own Hash, which then no longer matches what the NEXT receipt
// recorded as PrevHash — tampering with history breaks the chain visibly,
// the same idea a real blockchain uses.
//
// Fields exported, like User's (not like Sale's): a receipt is a data
// record you might want %+v or JSON on someday, not internal-only state
// the way Sale's mutex-guarded counter is.
type Receipt struct { // phase-8
	PrevHash        [32]byte // phase-8
	Buyer           string   // phase-8
	TokenAmount     int64    // phase-8
	TotalTokensSold int64    // phase-8: Sale's state at the moment of this buy, baked into the hash too
	Hash            [32]byte // phase-8: sha256 of everything above — this receipt's own identity
} // phase-8

// newReceipt builds the next link, given the previous one's hash (or the
// zero value, for the first receipt in a chain).
func newReceipt(prevHash [32]byte, buyer string, tokenAmount, totalTokensSold int64) Receipt { // phase-8
	r := Receipt{ // phase-8
		PrevHash:        prevHash,        // phase-8
		Buyer:           buyer,           // phase-8
		TokenAmount:     tokenAmount,     // phase-8
		TotalTokensSold: totalTokensSold, // phase-8
	} // phase-8
	// Covers PrevHash plus every field above it. %x/%d with colon
	// separators is unambiguous enough for this demo since none of these
	// fields can themselves contain a colon; a real system would want a
	// fixed-width or length-prefixed encoding instead of relying on that.
	data := fmt.Sprintf("%x:%s:%d:%d", r.PrevHash, r.Buyer, r.TokenAmount, r.TotalTokensSold) // phase-8
	r.Hash = sha256.Sum256([]byte(data))                                                      // phase-8
	return r                                                                                  // phase-8
} // phase-8

// verifyChain walks receipts and confirms each one's Hash really is
// sha256 of its own fields, and that each PrevHash really matches the
// previous receipt's Hash. Returns the index of the first broken link, or
// -1 if the whole chain is intact.
func verifyChain(receipts []Receipt) int { // phase-8
	var prevHash [32]byte        // phase-8: zero value — the first receipt's PrevHash must be all-zero
	for i, r := range receipts { // phase-8: range-by-value is fine — this only ever reads
		if r.PrevHash != prevHash { // phase-8
			return i // phase-8
		} // phase-8
		want := newReceipt(r.PrevHash, r.Buyer, r.TokenAmount, r.TotalTokensSold) // phase-8: recompute — never trust r.Hash itself, derive the true one
		if r.Hash != want.Hash {                                                  // phase-8
			return i // phase-8
		} // phase-8
		prevHash = r.Hash // phase-8
	} // phase-8
	return -1 // phase-8
} // phase-8

// startingBalanceTokens is the flag target for -balance, in WHOLE tokens —
// friendlier on a command line than typing a base-unit count. startingBalance
// itself (base units) is still computed from this × Unit after flag.Parse(),
// same "derive from Unit, never type out the literal" rule Phase 1 used
// (main.go:26-27).
var startingBalanceTokens = int64(100) // phase-7

// main walks the checkpoints in order.
func main() {
	// Phase 7: CLI flags. Must run before anything below reads numUsers,
	// startingBalance, basePrice, or curveK — command-line values only land
	// in these variables once Parse actually executes; the third argument
	// to each call below is only the default shown in -h, not a live value.
	flag.IntVar(&numUsers, "users", numUsers, "number of simulated users")                                                // phase-7
	flag.Int64Var(&startingBalanceTokens, "balance", startingBalanceTokens, "starting balance per user, in whole tokens") // phase-7
	flag.Float64Var(&basePrice, "price", basePrice, "curve base price (Phase 7+ checkpoints only)")                       // phase-7
	flag.Float64Var(&curveK, "k", curveK, "curve growth rate per base unit (Phase 7+ checkpoints only)")                  // phase-7
	flag.Parse()                                                                                                          // phase-7
	startingBalance = startingBalanceTokens * Unit                                                                        // phase-7: recomputed from the (possibly flag-overridden) whole-token value

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

	// Phase 5 checkpoint, part one: the range-copy trap, live, on Buy instead
	// of fundByValue. Same underlying mechanism as Phase 3a, different method.
	fmt.Println("\n== Phase 5a: the range-copy trap, on Buy this time ==")             // phase-5
	buyAmount := 30 * Unit                                                             // phase-5: var, not const — used again below with a different meaning if changed later
	fmt.Printf("  before: %s tokens (users[0])\n", formatUnits(users[0].TokenBalance)) // phase-5
	for _, u := range users {                                                          // phase-5: range copies each User into u — u is NOT users[i]
		// u is addressable (it's a real local variable), so Go can take &u
		// automatically for the pointer receiver. u.Buy(...) compiles and
		// returns a nil error — it genuinely succeeds. It just succeeds
		// against the copy. This is what makes the bug dangerous: nothing
		// here looks wrong, not even the error return.
		if err := u.Buy(buyAmount); err != nil { // phase-5: pointer receiver call on a copy — mutates the copy only
			fmt.Printf("  %s: buy failed: %v\n", u.Name, err) // phase-5
		} // phase-5
	} // phase-5
	fmt.Printf("  after:  %s tokens (users[0])  <- unchanged, Buy ran on a copy\n", // phase-5
		formatUnits(users[0].TokenBalance))

	// Phase 5 checkpoint, part two: buyAll does the real, index-based buying
	// (the mechanism); this loop only decides how to react to each result
	// (the policy). Also sets up a genuine insufficient-funds case: drain
	// users[2] for real before the uniform buy, so its later failure in the
	// batch isn't a contrived error path.
	fmt.Println("\n== Phase 5b: buyAll (mechanism) + a policy loop, and a buyer who can't afford it ==") // phase-5
	if err := users[2].Buy(80 * Unit); err != nil {                                                      // phase-5: users[2] is the real element, no range involved
		fmt.Printf("  setup buy failed: %v\n", err) // phase-5
	} // phase-5
	results := buyAll(users, buyAmount) // phase-5: one call, one []error back — users is mutated in place by now
	for i, err := range results {       // phase-5: ranging over []error, not []User — no copy-of-User trap here, there's no User in this slice
		if err != nil { // phase-5
			// Skip and continue, not stop everything: one buyer's failure
			// isn't the whole sale's problem, and Buy already validated
			// before mutating, so a failed buyer is left untouched either
			// way — nothing needs to be rolled back before moving on.
			fmt.Printf("  %s: buy failed: %v\n", users[i].Name, err) // phase-5: users[i] still valid — same length, same order as results
			continue                                                 // phase-5: explicit — skips the success branch below for this i
		} // phase-5
		fmt.Printf("  %s: bought %s tokens\n", users[i].Name, formatUnits(buyAmount)) // phase-5
	} // phase-5

	fmt.Println("\n== Phase 5 final summary ==") // phase-5
	for i := range users {                       // phase-5: index again, purely for consistency with the loop above — a value range would read fine too
		printSummary(users[i]) // phase-5: read-only call, same as the Phase 4 print loops
	} // phase-5

	// Phase 7 checkpoint, part one: Sale + the exponential curve, still
	// sequential (concurrency is the next slice of Phase 7, not this one).
	// Fresh users and a fresh Sale, so this demo isn't reading balances the
	// earlier phases already changed. Same fixed buyAmount as Phase 5, but
	// unlike buyAll — where every buyer priced independently off the flat
	// PriceBaseUnits — every buy here reads and moves the same
	// sale.totalTokensSold, so price visibly climbs from one buyer to the
	// next.
	fmt.Println("\n== Phase 7a: Sale + the exponential curve ==") // phase-7
	curveUsers := newUsers(4, startingBalance)                    // phase-7
	var sale Sale                                                 // phase-7: zero value is ready to use — totalTokensSold 0, mu unlocked, no constructor needed
	for i := range curveUsers {                                   // phase-7: index-only range — same range-copy lesson from Phase 5 still applies
		// price() here is just for display, read before the buy moves
		// totalTokensSold — Buy recomputes the same thing internally under
		// its own lock, this call isn't racing anything.
		quoted := price(sale.totalTokensSold) // phase-7
		// sale.Buy(...): sale is the one real local variable from `var sale
		// Sale` above, not a range copy, so Go's auto-&sale for the pointer
		// receiver lands on the actual Sale — the Phase 3b/alice case, not
		// the Phase 3a/5a copy trap. &curveUsers[i]: Sale.Buy takes *User,
		// and curveUsers[i] is the real element (indexed, not
		// ranged-by-value) — same addressing rule buyAll relied on in
		// Phase 5.
		if err := sale.Buy(&curveUsers[i], buyAmount); err != nil { // phase-7
			fmt.Printf("  %s: buy failed: %v\n", curveUsers[i].Name, err) // phase-7
			continue                                                      // phase-7
		} // phase-7
		// quoted is a price RATIO (cost per token, both already in base
		// units), not itself a quantity of base units — formatUnits would
		// wrongly divide it by Unit again, so it's printed directly instead.
		fmt.Printf("  %s: price was %.6f, bought %s tokens, sold so far: %s\n", // phase-7
			curveUsers[i].Name, quoted, formatUnits(buyAmount), formatUnits(sale.totalTokensSold))
	} // phase-7

	// Phase 7 checkpoint, part two: Buyer, the interface. &curveUsers[0]
	// satisfies Buyer because *User has a matching Buy(int64) error method
	// — nothing declares that on purpose, Go interfaces are implicit.
	// attemptPurchase never mentions User by name, only Buyer.
	fmt.Println("\n== Phase 7b: Buyer, the interface ==") // phase-7
	var b Buyer = &curveUsers[0]                          // phase-7: compiles only because *User's method set includes Buy(int64) error
	if err := attemptPurchase(b, buyAmount); err != nil { // phase-7
		fmt.Printf("  %s: buy failed: %v\n", curveUsers[0].Name, err) // phase-7
	} else { // phase-7
		fmt.Printf("  %s bought through attemptPurchase(Buyer, ...), no *User in that function's signature\n", // phase-7
			curveUsers[0].Name)
	} // phase-7

	// Phase 7 checkpoint, part three: the race, live. newUserMap for the
	// maps stretch goal, doubling as the buyer pool every goroutine below
	// draws from.
	fmt.Println("\n== Phase 7c: the race, live (no lock) ==") // phase-7
	raceUsers := newUserMap(1000, startingBalance)            // phase-7: map[string]*User — enough concurrent goroutines to make the race reliably visible
	raceAmount := 1 * Unit                                    // phase-7
	var unsafeSale Sale                                       // phase-7
	var wg sync.WaitGroup                                     // phase-7
	for _, u := range raceUsers {                             // phase-7: range over a map — u is already *User, no copy-of-User trap possible here
		wg.Add(1)   // phase-7: Add before the goroutine starts, never inside it — Add itself isn't safe to race against Wait
		go func() { // phase-7: closes over u directly — safe because go.mod targets 1.26.5, and Go 1.22+ gives every range iteration its own u. Before 1.22 this closure would have shared one mutating u across all goroutines, mostly seeing the last iteration's value by the time they ran.
			defer wg.Done()                         // phase-7
			_ = unsafeSale.unsafeBuy(u, raceAmount) // phase-7: error deliberately discarded — concurrent prints would interleave into noise; the totals below are the actual point
		}() // phase-7
	} // phase-7
	wg.Wait()                                                                               // phase-7: blocks until every goroutine above has called Done — nothing after this line races with the goroutines
	expected := int64(len(raceUsers)) * raceAmount                                          // phase-7
	fmt.Printf("  expected totalTokensSold: %s\n", formatUnits(expected))                   // phase-7
	fmt.Printf("  actual   totalTokensSold: %s\n", formatUnits(unsafeSale.totalTokensSold)) // phase-7
	if unsafeSale.totalTokensSold != expected {                                             // phase-7
		fmt.Println("  mismatch — lost updates from the missing lock.") // phase-7
	} else { // phase-7
		fmt.Println("  numbers matched this run anyway — races are exactly this unreliable to observe by eye.") // phase-7
	} // phase-7
	fmt.Println("  `go run -race .` catches the data race itself, deterministically, even on a run where the numbers happen to match.") // phase-7

	// Phase 7 checkpoint, part four: same shape, Sale.Buy this time —
	// mutex-protected. Only the method name differs from Phase 7c.
	fmt.Println("\n== Phase 7d: same race, Sale.Buy this time (locked) ==") // phase-7
	safeUsers := newUserMap(1000, startingBalance)                          // phase-7
	var safeSale Sale                                                       // phase-7
	var wg2 sync.WaitGroup                                                  // phase-7
	for _, u := range safeUsers {                                           // phase-7
		wg2.Add(1)  // phase-7
		go func() { // phase-7
			defer wg2.Done()                // phase-7
			_ = safeSale.Buy(u, raceAmount) // phase-7: identical call shape to unsafeBuy above — Lock()/Unlock() inside is the only difference
		}() // phase-7
	} // phase-7
	wg2.Wait()                                                                          // phase-7
	fmt.Printf("  expected totalTokensSold: %s\n", formatUnits(expected))               // phase-7
	fmt.Printf("  actual   totalTokensSold: %s   <- always matches, lock or no luck\n", // phase-7
		formatUnits(safeSale.totalTokensSold))

	// Phase 8 checkpoint, part one: ed25519 identity + signed buys.
	fmt.Println("\n== Phase 8a: ed25519 identity + signed buys ==")       // phase-8
	signedUser, signedPriv, err := newSignedUser("dave", startingBalance) // phase-8
	if err != nil {                                                       // phase-8
		fmt.Printf("  keypair generation failed: %v\n", err) // phase-8
	} else { // phase-8
		var signedSale Sale                                                           // phase-8
		msg := signBuyMessage(signedUser.Name, buyAmount)                             // phase-8
		validSig := ed25519.Sign(signedPriv, msg)                                     // phase-8: signing happens here, with the private key, never inside SignedBuy — same as a real wallet signing locally before submitting
		if err := signedSale.SignedBuy(signedUser, buyAmount, validSig); err != nil { // phase-8
			fmt.Printf("  valid signature rejected (bug): %v\n", err) // phase-8
		} else { // phase-8
			fmt.Printf("  %s: valid signature accepted, bought %s tokens\n", signedUser.Name, formatUnits(buyAmount)) // phase-8
		} // phase-8

		// Same request, deliberately corrupted signature — proof forgery
		// doesn't work even against a real, already-successful buyer.
		forgedSig := make([]byte, len(validSig))                                       // phase-8
		copy(forgedSig, validSig)                                                      // phase-8
		forgedSig[0] ^= 0xFF                                                           // phase-8: flip one bit — ed25519 signatures don't degrade gracefully, this is enough to invalidate it completely
		if err := signedSale.SignedBuy(signedUser, buyAmount, forgedSig); err != nil { // phase-8
			fmt.Printf("  forged signature correctly rejected: %v\n", err) // phase-8
		} else { // phase-8
			fmt.Println("  forged signature accepted (bug!)") // phase-8
		} // phase-8
	} // phase-8

	// Phase 8 checkpoint, part two: the hash-chained receipt log.
	fmt.Println("\n== Phase 8b: hash-chained receipts ==") // phase-8
	var receipts []Receipt                                 // phase-8: nil slice — append grows it, same reasoning newUsers gave for starting empty in Phase 4
	var chainSale Sale                                     // phase-8
	chainUsers := newUsers(3, startingBalance)             // phase-8
	for i := range chainUsers {                            // phase-8: index-only range — the same lesson Phase 5 already taught still applies
		if err := chainSale.Buy(&chainUsers[i], buyAmount); err != nil { // phase-8
			fmt.Printf("  %s: buy failed: %v\n", chainUsers[i].Name, err) // phase-8
			continue                                                      // phase-8
		} // phase-8
		var prevHash [32]byte  // phase-8
		if len(receipts) > 0 { // phase-8
			prevHash = receipts[len(receipts)-1].Hash // phase-8: chain to the actual previous receipt, not assumed order
		} // phase-8
		receipts = append(receipts, newReceipt(prevHash, chainUsers[i].Name, buyAmount, chainSale.totalTokensSold)) // phase-8
	} // phase-8
	fmt.Printf("  %d receipts recorded, chain intact: %t\n", len(receipts), verifyChain(receipts) == -1) // phase-8

	if len(receipts) > 0 { // phase-8
		// Tamper with history after the fact — exactly what an attacker
		// would try — without recomputing its hash, and prove verifyChain
		// catches it.
		receipts[0].TokenAmount = 999 * Unit                                                        // phase-8
		broken := verifyChain(receipts)                                                             // phase-8
		fmt.Printf("  after tampering with receipt 0: chain intact = %t, first broken link = %d\n", // phase-8
			broken == -1, broken)
	} // phase-8

	fmt.Println("\nAll 8 phases complete: Phases 1-6 (fundamentals), Phase 7 (shared state,")
	fmt.Println("the exponential curve, maps, interfaces, CLI flags, concurrency), and Phase 8")
	fmt.Println("(ed25519 signed buys, sha256 hash-chained receipts). See main_test.go for the")
	fmt.Println("table-driven tests (run with `go test ./...`, not part of this program's own output).")
}
