package builder

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

var poolAddr = common.HexToAddress("0x1111111111111111111111111111111111111111")

// v2log builds a V2 Swap(amount0In, amount1In, amount0Out, amount1Out). buy(token0->token1) = a0In>0,a1Out>0.
func v2log(a0in, a1in, a0out, a1out int64) *types.Log {
	d := make([]byte, 128)
	new(big.Int).SetInt64(a0in).FillBytes(d[0:32])
	new(big.Int).SetInt64(a1in).FillBytes(d[32:64])
	new(big.Int).SetInt64(a0out).FillBytes(d[64:96])
	new(big.Int).SetInt64(a1out).FillBytes(d[96:128])
	return &types.Log{Address: poolAddr, Topics: []common.Hash{common.HexToHash("0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822")}, Data: d}
}
func v2buy() *types.Log  { return v2log(1e18, 0, 0, 1e18) } // token0 in, token1 out -> true
func v2sell() *types.Log { return v2log(0, 1e18, 1e18, 0) } // token1 in, token0 out -> false

func TestHasSandwichPattern(t *testing.T) {
	cases := []struct {
		d    []bool
		want bool
	}{
		{[]bool{true, true, false}, true},
		{[]bool{false, false, true}, true},
		{[]bool{true, false, true, false}, true},
		{[]bool{true, false}, false},
		{[]bool{true, false, false}, false},
		{[]bool{false, true, true}, false},
		{[]bool{true, true, true}, false},
	}
	for _, c := range cases {
		if got := hasSandwichPattern(c.d); got != c.want {
			t.Errorf("hasSandwichPattern(%v)=%v want %v", c.d, got, c.want)
		}
	}
}

func TestDetectBundle(t *testing.T) {
	buy := []*types.Log{v2buy()}
	sell := []*types.Log{v2sell()}
	if DetectSandwichForBundle([][]*types.Log{buy, buy, sell}) == nil {
		t.Error("expected sandwich for buy,buy,sell")
	}
	if DetectSandwichForBundle([][]*types.Log{buy, sell}) != nil {
		t.Error("2-tx backrun should pass")
	}
	multi := []*types.Log{v2buy(), v2buy()}
	if DetectSandwichForBundle([][]*types.Log{multi, sell}) != nil {
		t.Error("multihop collapse should avoid false positive")
	}
}
