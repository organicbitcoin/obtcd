package reap

import (
	"context"
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/blockchain/expiryindex"
)

func benchmarkSelect(b *testing.B, n int) {
	view := blockchain.NewUtxoViewpoint()
	scanner := &stubScanner{}
	for i := 0; i < n; i++ {
		op := addUtxo(b, view, int64(1000+i%100), uint32(i))
		scanner.items = append(scanner.items, &expiryindex.ExpiringUTXO{OutPoint: op, ExpiryKey: uint64(10 + i%500)})
	}
	p := DefaultREAPParams(SortModeStrict)
	p.MaxInputs = n
	p.ScanBatch = 2048
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := selectCandidatesWithScanner(context.Background(), 10_000, scanner, view, p)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSelectCandidates1k(b *testing.B)  { benchmarkSelect(b, 1000) }
func BenchmarkSelectCandidates10k(b *testing.B) { benchmarkSelect(b, 10000) }
