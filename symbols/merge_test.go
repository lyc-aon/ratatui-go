package symbols

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

// This corpus was generated from Ratatui 0.30.2 at UpstreamRevision and covers
// every pair of Unicode box-drawing characters under all merge strategies.
func TestMergeSymbolUpstreamCorpus(t *testing.T) {
	symbols := []string{" ", "x"}
	for r := rune(0x2500); r <= 0x257f; r++ {
		symbols = append(symbols, string(r))
	}
	strategies := []MergeStrategy{
		MergeStrategyReplace,
		MergeStrategyExact,
		MergeStrategyFuzzy,
	}

	hash := sha256.New()
	for strategyIndex, strategy := range strategies {
		for previousIndex, previous := range symbols {
			for nextIndex, next := range symbols {
				merged := []rune(strategy.Merge(previous, next))
				var value rune
				if len(merged) != 0 {
					value = merged[0]
				}
				_, _ = fmt.Fprintf(hash, "%d %d %d %x\n", strategyIndex, previousIndex, nextIndex, value)
			}
		}
	}

	const want = "bcf10dd411c848c18f2b1b98388b5f5f2c28bc4363fd986b59dafd2bec8dda37"
	if got := fmt.Sprintf("%x", hash.Sum(nil)); got != want {
		t.Fatalf("merge corpus hash = %s, want %s", got, want)
	}
}
