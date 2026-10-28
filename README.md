# BSC Sandwich Filter

A Go library for detecting directional sandwich patterns in an ordered bundle
of transaction execution logs. It performs no RPC calls and sends no transactions.

## Detection rule

`DetectSandwichForBundle(bundleLogs [][]*types.Log) error` examines transactions
in bundle order. For each transaction and pool, swap events are reduced to one
direction by event-count majority; ties contribute no direction. The filter
rejects when any pool contains a **buy-buy-sell** or **sell-sell-buy** subsequence
at three distinct transaction positions. Positions need not be adjacent.
"Buy" and "sell" denote opposite token directions, not fiat value or intent.
Two-transaction bundles and same-transaction round trips cannot alone form this
three-position pattern.

## Recognized events

- V2-compatible swaps: two event signatures; direction uses all four amounts.
- V3-compatible swaps: three signatures; signed amount comparison.
- V4 / PancakeSwap Infinity swaps: two signatures; indexed pool ID and signed amount.
- DODO swaps: one signature; token-pair key and address ordering.
- FourMeme buys and sells: two signatures; token key and event direction.

The ten signature constants match the public
[48Club/bscexorcist event registry](https://github.com/48Club/bscexorcist/blob/main/protocols/swap.go).
That project is a reference, not a dependency; matching event signatures does
not claim complete behavioral equivalence.

## Use

Requires Go 1.23 or newer. Supply complete, well-formed execution logs, with no
nil log pointers, grouped by transaction in execution order. The Go package
name is `builder`; an import alias is convenient:

```go
import sandwich "github.com/zeminus/bsc-sandwich-filter"

// bundleLogs has type [][]*types.Log.
if err := sandwich.DetectSandwichForBundle(bundleLogs); err != nil {
    // Reject the complete bundle.
}
```

Run `go test ./...` and `go test -race ./...`.

## Scope and limitations

This is a log-pattern heuristic, not proof of attacker identity or profitability.
It cannot detect unknown event signatures, omitted logs, or patterns spanning
separate bundles or public transactions. Unrelated strategies can match the
same directional pattern. A nil result means only that this pattern was absent.

Pool keys follow the implementation: V2/V3 use the emitter address; V4 uses the
indexed pool ID (or emitter fallback), without manager-address namespacing;
DODO combines ten-byte prefixes of sorted token addresses; FourMeme uses the
token address. These keys can conflate distinct pools. Event counts are not
volume-weighted. Input validation and bundle ordering belong to the caller.

## License

This repository's filter and tests are MIT licensed; see [LICENSE](LICENSE).
Dependencies retain their own licenses. No third-party dependency source is
vendored here.
