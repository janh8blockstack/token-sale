# token sale

A token sale simulation in Go, built phase by phase. **Phases 1 to 4 are done.**

```sh
go run .
```

## About the pump.fun bonding curve

The starting assumption was "exponential bonding curve". That's not what pump.fun uses.

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

So Phases 1 to 4 use a flat 1:1 price, with the types chosen so the curve drops
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

### The Phase 5 trap, in advance

Phase 4's print loop uses `for _, u := range users`, which is safe because
`printSummary` only reads. Phase 5 calls `Buy` in that same loop, and `u` is a
**copy** of the slice element. It's the same trap `fundByValue` demonstrates in
Phase 3a, wearing different clothes. Index the slice instead
(`users[i].Buy(...)`) or range over a `[]*User`. Worth deriving from the Phase 3a
output rather than reading here.

### Where the curve goes

Phase 7 stretch goal 2 (a `Sale` struct holding shared state) is the natural
home. Put `virtualSolReserves` and `virtualTokenReserves` on it, replace the
`PriceBaseUnits` constant with
`func (s *Sale) BuyTokens(u *User, solIn int64) error`, and the constant product
formula above becomes two lines of integer arithmetic. `Buy` moving from `*User`
to `*Sale` is the point: the price stops being a property of the buyer and
becomes a property of the market.

## Sources

- [pump.fun bonding curve docs](https://pump.fun/docs/bonding-curve)
- [Bonding Curve Mechanism, pump-public-docs](https://deepwiki.com/pump-fun/pump-public-docs/3.1-pump-bonding-curve-mechanism)
- [pump-fun-sdk bonding-curve-math.md](https://github.com/nirholas/pump-fun-sdk/blob/main/docs/bonding-curve-math.md)
- [Pump.fun Bonding Curve Mechanics Explained](https://flashift.app/blog/bonding-curves-pump-fun-meme-coin-launches/)
