# token sale

A token sale simulation in Go, built phase by phase. **Phases 1 to 4 are done.**

```sh
go run .
```

## Architecture

![Architecture: the User and Sale data model, the Buy purchase pipeline with its exponential pricing formula, and the mutex-guarded concurrency section](architecture.svg)

Shows where the project stands (`User`, done, Phase 2) alongside where Phase 7
is headed (`Sale`, the exponential pricing formula, and the `sync.Mutex`
critical section) — see the phase map below for what's actually built today.

## About the pump.fun bonding curve

The starting assumption was "exponential bonding curve". That's not what pump.fun
uses. This section is background on pump.fun's real mechanism, not a description
of what this project builds — see "Where the curve goes" below for that.

pump.fun runs a constant product AMM (Uniswap V2, `x·y = k`) over *virtual* reserves:

| | value |
|---|---|
| invariant | `virtual_token_reserves × virtual_sol_reserves = k` |
| initial `virtual_sol_reserves` | `30_000_000_000` lamports (30 SOL) |
| initial `virtual_token_reserves` | `1_073_000_000_000_000` base units |
| initial `real_token_reserves` | `793_100_000_000_000` base units |
| `token_total_supply` | `1_000_000_000_000_000` base units |
| graduation | when `real_token_reserves == 0`, migrates to PumpSwap AMM |

Buy, given SOL in (after fees):

```
tokensOut = (solIn × virtualTokenReserves) / (virtualSolReserves + solIn)
```

Sell, given tokens in:

```
solOut = (tokenAmount × virtualSolReserves) / (virtualTokenReserves + tokenAmount)
```

The "virtual" part is the trick. The curve is seeded with 30 SOL that nobody
deposited, purely so the first buyer doesn't get tokens at a price of zero.

**Why it looks exponential.** Price is `virtualSol / virtualToken`. As tokens are
bought the denominator shrinks toward zero, so price rises *hyperbolically*, a
`1/(1-x)` shape that steepens without bound near graduation. Plotted on a chart
that reads as an exponential blow off, which is why most blog posts call it one.
It isn't. The math is a hyperbola, and the distinction matters the moment you try
to invert the formula to answer "how much SOL to buy X tokens".

### Why the curve is not in Phases 1 to 4

Phase 1 asks for "a constant for the token price (start at 1:1)", and Phases 2 to
4 never move it. A curve is not a price. It's mutable reserve state shared by
every buyer, which is exactly the `Sale` struct from Phase 7 stretch goal 2.
Building it now would mean skipping the receiver lesson to get there.

So Phases 1 to 4 use a flat 1:1 price, with the types chosen so a curve drops
in later.

## Design decisions worth knowing

**Money is `int64` base units, never `float64`.** `float64` cannot represent `0.1`
exactly, so repeated additions drift and balances stop summing to the total
supply. Every real chain stores integer base units and divides only for display.
A lamport is exactly that. `Unit = 1_000_000` (6 decimals, same as pump.fun
tokens), and `formatUnits` is the only place a decimal point exists.

At a 1:1 price nothing rounds, so Phases 1 to 4 are exact. That stops being true
the moment the curve lands: integer division truncates, and *which way you round*
is a real decision. Round in the protocol's favour, never the buyer's. Rounding
up on cost and down on tokens out is what keeps the invariant from leaking value.

**`Buy` returns an `error` instead of printing.** Only the caller knows what a
failure means. Phase 5 skips that buyer and continues, a test asserts on it, a CLI
exits non zero. A method that prints has decided for all of them and thrown the
information away. Failures are sentinel wrapped, so callers use
`errors.Is(err, ErrInsufficientFunds)` rather than matching message text.

**`Buy` validates before it mutates.** A rejected purchase leaves the user
unchanged, byte for byte. Deduct then validate is how you get negative balances.

## Phase map

| phase | what | status |
|---|---|---|
| 1 | variables and constants | done |
| 2 | `User` struct, `%+v` | done |
| 3 | methods, value vs pointer receiver, errors | done |
| 4 | slices and loops, `append` vs pre sized | done |
| 5 | everyone buys, the `range` copy gotcha | next |
| 6 | self review (`gofmt`, `go vet`) | ongoing, clean |
| 7 | maps, `Sale` state, interfaces, tests, flags, concurrency | later |

## Phase-by-phase: lines and concepts

Line numbers are current as of `main.go` on this branch; they'll drift as Phase
5+ code gets added above/below.

### Phase 1 — variables and constants (`main.go:10-30`)

| block | lines |
|---|---|
| `const` (`Decimals`, `Unit`, `PriceBaseUnits`) | 12-21 |
| `var` (`numUsers`, `startingBalance`) | 27-30 |

Concepts: package declaration and imports · grouped `const`/`var` blocks ·
`const` vs `var` (compiler-enforced immutability for the price) · untyped vs
explicitly-typed constants · explicit type conversion (`int64(...)`) ·
numeric literal underscores (`1_000_000`) · type inference on `var` ·
package-level scope (why `:=` isn't legal here, only inside functions) ·
derived constants (`startingBalance` computed from `Unit`, not hardcoded).

### Phase 2 — the `User` struct (`main.go:32-48`, checkpoint `main.go:186-195`)

Concepts: `type ... struct { ... }` declaration · struct fields as
`name type` pairs · exported vs unexported identifiers via capitalization ·
why capitalization matters for `%+v` (reflection can't see unexported
fields) · `int64` over `float64` for money · named-field struct literals ·
`:=` short variable declaration (legal here, illegal at package scope) ·
the `%+v` formatting verb · passing a struct by value into a read-only
function (`printSummary`).

### Phase 3 — methods, functions, errors (`main.go:50-130`, checkpoint `main.go:197-227`)

Concepts: sentinel errors via `errors.New` · methods and receivers ·
pointer vs value receivers, the core lesson (`fundByValue` mutates a copy
and does nothing observable; `Fund`/`Buy` use `*User` and actually mutate) ·
the `if err := f(); err != nil` idiom · error wrapping with `%w` and
`errors.Is` · `error` as the conventional last return value · validate
before mutate (a rejected `Buy` leaves the user untouched) · `fmt` verbs
including a variable-width format (`%0*d`) · local `:=`/`=` inside a
function body.

### Phase 4 — slices and loops (`main.go:132-179`, checkpoint `main.go:229-242`)

Concepts: slices (`[]User`) · `make` in both forms — `make([]User, 0, n)`
(zero length, pre-reserved capacity, for `append`) vs `make([]User, n)`
(n zero-valued elements up front, for index assignment) · `append` growth ·
the append-after-presized-make trap (flagged in the code comment) ·
`for i := range n` (ranging over a bare int, Go 1.22+) · `for i := range
users` (index-only range) · `for _, u := range users` (blank identifier,
and the range-copy semantics that Phase 5 will have to stop relying on) ·
struct literals inside a loop with an omitted field taking its zero value ·
`fmt.Sprintf` for building names (`user%02d`) · guard clauses
(`if n < 0 { n = 0 }`).

### The Phase 5 trap, in advance

Phase 4's print loop uses `for _, u := range users`, which is safe because
`printSummary` only reads. Phase 5 calls `Buy` in that same loop, and `u` is a
**copy** of the slice element. It's the same trap `fundByValue` demonstrates in
Phase 3a, wearing different clothes. Index the slice instead
(`users[i].Buy(...)`) or range over a `[]*User`. Worth deriving from the Phase 3a
output rather than reading here.

### Where the curve goes

Phase 7 stretch goal 2 (a `Sale` struct holding shared state) is the natural
home for pricing to leave `PriceBaseUnits` behind — but not with the
constant-product formula above. This project's curve is a true exponential
curve, `price(supply) = basePrice × e^(k × totalTokensSold)`: inspired by the
same "early buyers pay less" shape as pump.fun, implemented with different,
simpler math. It is not a reimplementation of pump.fun's actual contract.

`Sale` holds `totalTokensSold`; `Buy` moves from `*User` to
`func (s *Sale) Buy(u *User, tokenAmount int64) error`, pricing off that shared
counter. `Buy` moving from `*User` to `*Sale` is the point: the price stops
being a property of the buyer and becomes a property of the market.

`e^x` has no exact integer form, so the curve math needs `float64` internally,
rounded to `int64` base units only at the boundary, right before it touches a
stored balance — the one deliberate exception to the int64-only rule below.

## Sources

- [pump.fun bonding curve docs](https://pump.fun/docs/bonding-curve)
- [Bonding Curve Mechanism, pump-public-docs](https://deepwiki.com/pump-fun/pump-public-docs/3.1-pump-bonding-curve-mechanism)
- [pump-fun-sdk bonding-curve-math.md](https://github.com/nirholas/pump-fun-sdk/blob/main/docs/bonding-curve-math.md)
- [Pump.fun Bonding Curve Mechanics Explained](https://flashift.app/blog/bonding-curves-pump-fun-meme-coin-launches/)
