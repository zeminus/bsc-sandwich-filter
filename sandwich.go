// Package builder detects directional sandwich patterns in ordered bundle logs.
package builder

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// Recognized swap event signatures; see README.md for the public reference.
var (
	v2Swap = map[common.Hash]bool{
		common.HexToHash("0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822"): true,
		common.HexToHash("0x606ecd02b3e3b4778f8e97b2e03351de14224efaa5fa64e62200afc9395c2499"): true,
	}
	v3Swap = map[common.Hash]bool{
		common.HexToHash("0xc42079f94a6350d7e6235f29174924f928cc2ac818eb64fed8004e115fbcca67"): true,
		common.HexToHash("0x19b47279256b2a23a1665c810c8d55a1758940ee09377d4f8d26497a3577dc83"): true,
		common.HexToHash("0x121cb44ee54098b1a04743c487e7460d8dd429b27f88b1f4d4767396e1a59f79"): true,
	}
	v4Swap = map[common.Hash]bool{
		common.HexToHash("0x40e9cecb9f5f1f1c5b9c97dec2917b7ee92e57ba5563708daca94dd84ad7112f"): true,
		common.HexToHash("0x04206ad2b7c0f463bff3dd4f33c5735b0f2957a351e4f79763a4fa9e775dd237"): true,
	}
	dodoSwap     = common.HexToHash("0xc2c0245e056d5fb095f04cd6373bc770802ebd1e6c918eb78fdef843cdb37b0f")
	fourMemeBuy  = common.HexToHash("0x7db52723a3b2cdd6164364b3b766e65e540d7be48ffa89582956d8eaebe62942")
	fourMemeSell = common.HexToHash("0x0a5575b3648bae2210cee56bf33254cc1ddfbc7bf637c0af2ac18b14fb1bae19")
	twoExp256    = new(big.Int).Lsh(big.NewInt(1), 256)
)

type swapDir struct {
	pool      common.Hash
	token0To1 bool
}

// word reads 32-byte word i from data as an unsigned big.Int (0 if out of range).
func word(data []byte, i int) *big.Int {
	lo, hi := 32*i, 32*(i+1)
	if len(data) < hi {
		return new(big.Int)
	}
	return new(big.Int).SetBytes(data[lo:hi])
}

// asInt256 reinterprets an unsigned 256-bit word as a signed int256.
func asInt256(w *big.Int) *big.Int {
	if w.Bit(255) == 1 {
		return new(big.Int).Sub(w, twoExp256)
	}
	return w
}

// addrFromWord extracts an address from the rightmost 20 bytes of word i (go BytesToAddress).
func addrFromWord(data []byte, i int) common.Address {
	lo := 32 * i
	if len(data) < lo+32 {
		return common.Address{}
	}
	return common.BytesToAddress(data[lo : lo+32])
}

// dodoPoolKey: first 10 bytes of the numerically-smaller token ++ first 10 of the larger.
func dodoPoolKey(from, to common.Address) common.Hash {
	a, b := from, to
	if from.Big().Cmp(to.Big()) >= 0 {
		a, b = to, from
	}
	var pool common.Address
	copy(pool[:10], a.Bytes()[:10])
	copy(pool[10:], b.Bytes()[:10])
	return common.BytesToHash(pool.Bytes())
}

func parseSwaps(logs []*types.Log) []swapDir {
	var out []swapDir
	for _, l := range logs {
		if len(l.Topics) == 0 {
			continue
		}
		sig := l.Topics[0]
		switch {
		case v2Swap[sig]:
			if len(l.Data) < 128 {
				continue
			}
			// Swap(sender, amount0In, amount1In, amount0Out, amount1Out, to)
			// bscexorcist parity: token0->token1 iff token0 came in (delta0<0) and token1 went out (delta1>0),
			// using all four amounts rather than the amount0In>amount1In shortcut.
			a0in, a1in := word(l.Data, 0), word(l.Data, 1)
			a0out, a1out := word(l.Data, 2), word(l.Data, 3)
			delta0 := new(big.Int).Sub(a0out, a0in)
			delta1 := new(big.Int).Sub(a1out, a1in)
			out = append(out, swapDir{common.BytesToHash(l.Address.Bytes()), delta0.Sign() < 0 && delta1.Sign() > 0})
		case v3Swap[sig]:
			if len(l.Data) < 160 {
				continue
			}
			// bscexorcist parity: zeroForOne = amount0 > amount1 (both int256).
			a0, a1 := asInt256(word(l.Data, 0)), asInt256(word(l.Data, 1))
			out = append(out, swapDir{common.BytesToHash(l.Address.Bytes()), a0.Cmp(a1) > 0})
		case v4Swap[sig]:
			if len(l.Topics) != 3 || len(l.Data) < 64 {
				continue
			}
			pool := common.BytesToHash(l.Address.Bytes())
			if len(l.Topics) > 1 {
				pool = l.Topics[1] // v4/Infinity pool id is indexed
			}
			out = append(out, swapDir{pool, asInt256(word(l.Data, 0)).Sign() > 0})
		case sig == dodoSwap:
			if len(l.Topics) == 1 && len(l.Data) >= 128 {
				from, to := addrFromWord(l.Data, 0), addrFromWord(l.Data, 1)
				out = append(out, swapDir{dodoPoolKey(from, to), from.Big().Cmp(to.Big()) < 0})
			}
		case sig == fourMemeBuy || sig == fourMemeSell:
			if len(l.Topics) == 1 && len(l.Data) >= 32 {
				out = append(out, swapDir{common.BytesToHash(addrFromWord(l.Data, 0).Bytes()), sig == fourMemeBuy})
			}
		}
	}
	return out
}

func netDirectionsPerPool(logs []*types.Log) map[common.Hash]bool {
	type tally struct{ a, b int }
	t := map[common.Hash]*tally{}
	for _, d := range parseSwaps(logs) {
		if t[d.pool] == nil {
			t[d.pool] = &tally{}
		}
		if d.token0To1 {
			t[d.pool].a++
		} else {
			t[d.pool].b++
		}
	}
	out := make(map[common.Hash]bool, len(t))
	for p, c := range t {
		if c.a > c.b {
			out[p] = true
		} else if c.b > c.a {
			out[p] = false
		}
	}
	return out
}

func hasSandwichPattern(d []bool) bool {
	// A pattern exists when the current direction follows at least two
	// occurrences of the opposite direction. Counts saturate at two.
	var buys, sells uint8
	for _, buy := range d {
		if buy {
			if sells == 2 {
				return true
			}
			if buys < 2 {
				buys++
			}
		} else {
			if buys == 2 {
				return true
			}
			if sells < 2 {
				sells++
			}
		}
	}
	return false
}

// DetectSandwichForBundle returns an error naming the offending pool if the bundle (ordered per-tx
// logs) contains a Buy-Buy-Sell / Sell-Sell-Buy pattern on any pool. Nil = accept.
func DetectSandwichForBundle(bundleLogs [][]*types.Log) error {
	if len(bundleLogs) < 3 {
		return nil
	}
	perPool := map[common.Hash][]bool{}
	for _, txLogs := range bundleLogs {
		for pool, dir := range netDirectionsPerPool(txLogs) {
			perPool[pool] = append(perPool[pool], dir)
		}
	}
	for pool, dirs := range perPool {
		if hasSandwichPattern(dirs) {
			return fmt.Errorf("sandwich detected on pool %s", pool.Hex())
		}
	}
	return nil
}
