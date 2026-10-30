package builder

import "testing"

// Preserve the original exhaustive rule as an independent test oracle.
func referenceSandwichPattern(d []bool) bool {
	for i := 0; i < len(d)-2; i++ {
		for j := i + 1; j < len(d)-1; j++ {
			for k := j + 1; k < len(d); k++ {
				if d[i] == d[j] && d[j] != d[k] {
					return true
				}
			}
		}
	}
	return false
}

func TestSandwichPatternExhaustiveParity(t *testing.T) {
	for n := 0; n <= 16; n++ {
		d := make([]bool, n)
		for mask := 0; mask < 1<<n; mask++ {
			for i := range d {
				d[i] = mask&(1<<i) != 0
			}
			if got, want := hasSandwichPattern(d), referenceSandwichPattern(d); got != want {
				t.Fatalf("sequence %v: got %v want %v", d, got, want)
			}
		}
	}
}

var patternBenchmarkResult bool

func BenchmarkSandwichPatternLongUniform(b *testing.B) {
	d := make([]bool, 256)
	for _, tc := range []struct {
		name string
		fn   func([]bool) bool
	}{{"reference", referenceSandwichPattern}, {"current", hasSandwichPattern}} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				patternBenchmarkResult = tc.fn(d)
			}
		})
	}
}
