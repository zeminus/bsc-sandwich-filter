package builder

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func words(values ...int64) []byte {
	data := make([]byte, len(values)*32)
	for i, v := range values {
		w := big.NewInt(v)
		if v < 0 {
			w.Add(w, twoExp256)
		}
		w.FillBytes(data[i*32 : (i+1)*32])
	}
	return data
}

func TestRecognizedProtocolDirections(t *testing.T) {
	pool := common.HexToAddress("0x1111111111111111111111111111111111111111")
	id := common.HexToHash("0x1234")
	check := func(name string, buy, sell *types.Log, key common.Hash) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			for _, item := range []struct {
				log  *types.Log
				want bool
			}{{buy, true}, {sell, false}} {
				got := parseSwaps([]*types.Log{item.log})
				if len(got) != 1 || got[0].pool != key || got[0].token0To1 != item.want {
					t.Fatalf("unexpected parsed direction: %+v", got)
				}
			}
			for _, bundle := range [][][]*types.Log{
				{{buy}, {buy}, {sell}}, {{sell}, {sell}, {buy}},
			} {
				if DetectSandwichForBundle(bundle) == nil {
					t.Fatal("expected directional pattern rejection")
				}
			}
			if DetectSandwichForBundle([][]*types.Log{{buy}, {sell}}) != nil {
				t.Fatal("two transactions must not form the pattern")
			}
		})
	}
	for sig := range v2Swap {
		check("v2/"+sig.Hex(), &types.Log{Address: pool, Topics: []common.Hash{sig}, Data: words(10, 0, 0, 9)},
			&types.Log{Address: pool, Topics: []common.Hash{sig}, Data: words(0, 10, 9, 0)}, common.BytesToHash(pool.Bytes()))
	}
	for sig := range v3Swap {
		check("v3/"+sig.Hex(), &types.Log{Address: pool, Topics: []common.Hash{sig}, Data: words(10, -9)},
			&types.Log{Address: pool, Topics: []common.Hash{sig}, Data: words(-9, 10)}, common.BytesToHash(pool.Bytes()))
	}
	for sig := range v4Swap {
		check("v4/"+sig.Hex(), &types.Log{Address: pool, Topics: []common.Hash{sig, id}, Data: words(10, -9)},
			&types.Log{Address: pool, Topics: []common.Hash{sig, id}, Data: words(-9, 10)}, id)
	}
	a := common.HexToAddress("0x1111111111111111111111111111111111111111")
	b := common.HexToAddress("0x2222222222222222222222222222222222222222")
	dodoData := func(from, to common.Address) []byte {
		d := make([]byte, 128)
		copy(d[12:32], from.Bytes())
		copy(d[44:64], to.Bytes())
		return d
	}
	check("dodo", &types.Log{Address: pool, Topics: []common.Hash{dodoSwap}, Data: dodoData(a, b)},
		&types.Log{Address: pool, Topics: []common.Hash{dodoSwap}, Data: dodoData(b, a)}, dodoPoolKey(a, b))
	token := make([]byte, 32)
	copy(token[12:], a.Bytes())
	check("fourmeme", &types.Log{Address: pool, Topics: []common.Hash{fourMemeBuy}, Data: token},
		&types.Log{Address: pool, Topics: []common.Hash{fourMemeSell}, Data: token}, common.BytesToHash(a.Bytes()))
}

func TestTransactionCollapseAndPoolIsolation(t *testing.T) {
	buy, sell := v2buy(), v2sell()
	if len(netDirectionsPerPool([]*types.Log{buy, sell})) != 0 {
		t.Fatal("a tied transaction must contribute no direction")
	}
	if DetectSandwichForBundle([][]*types.Log{{buy, sell}, {buy}, {sell}}) != nil {
		t.Fatal("a tied first transaction must not create a third position")
	}
	other := *sell
	other.Address = common.HexToAddress("0x2222222222222222222222222222222222222222")
	if DetectSandwichForBundle([][]*types.Log{{buy}, {buy}, {&other}}) != nil {
		t.Fatal("different emitter pools must remain separate")
	}
	if DetectSandwichForBundle([][]*types.Log{{buy}, {}, {buy}, {sell}}) == nil {
		t.Fatal("nonadjacent positions must be checked")
	}
	if DetectSandwichForBundle([][]*types.Log{{buy, buy}, {sell}}) != nil {
		t.Fatal("two swaps in one transaction are not two positions")
	}
}

func TestUnsupportedAndProtocolSpecificMalformedLogs(t *testing.T) {
	logs := []*types.Log{
		{}, {Topics: []common.Hash{common.HexToHash("0xdeadbeef")}},
		{Topics: []common.Hash{dodoSwap}, Data: make([]byte, 127)},
		{Topics: []common.Hash{dodoSwap, common.Hash{}}, Data: make([]byte, 128)},
		{Topics: []common.Hash{fourMemeBuy}, Data: make([]byte, 31)},
		{Topics: []common.Hash{fourMemeSell, common.Hash{}}, Data: make([]byte, 32)},
	}
	if got := parseSwaps(logs); len(got) != 0 {
		t.Fatalf("unexpected recognized events: %+v", got)
	}
	if DetectSandwichForBundle(nil) != nil {
		t.Fatal("empty bundle must pass")
	}
}

func TestV2UsesAllFourAmounts(t *testing.T) {
	// amount0In > amount1In is not sufficient when token0 also leaves.
	got := parseSwaps([]*types.Log{v2log(10, 1, 11, 2)})
	if len(got) != 1 || got[0].token0To1 {
		t.Fatal("direction must use both net amount deltas")
	}
}
