// Copyright (c) 2025-2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	_ "github.com/btcsuite/btcd/database/ffldb"
	"github.com/btcsuite/btcd/wire"
)

// BenchmarkOutPointEncoding benchmarks OutPoint encoding/decoding
func BenchmarkOutPointEncoding(b *testing.B) {
	// Create test outpoint
	hash, _ := chainhash.NewHashFromStr("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	outpoint := &wire.OutPoint{
		Hash:  *hash,
		Index: 12345,
	}

	b.ResetTimer()

	b.Run("Encode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = encodeOutPoint(outpoint)
		}
	})

	encoded := encodeOutPoint(outpoint)
	b.Run("Decode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = decodeOutPoint(encoded)
		}
	})
}

// BenchmarkExpiryKeyEncoding benchmarks ExpiryKey encoding/decoding
func BenchmarkExpiryKeyEncoding(b *testing.B) {
	expiryKey := uint64(1000000)

	b.ResetTimer()

	b.Run("Encode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = encodeExpiryKey(expiryKey)
		}
	})

	encoded := encodeExpiryKey(expiryKey)
	b.Run("Decode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = decodeExpiryKey(encoded)
		}
	})
}

// BenchmarkOrderedOutPointEncoding benchmarks ordered OutPoint encoding/decoding.
func BenchmarkOrderedOutPointEncoding(b *testing.B) {
	hash := chainhash.Hash{}
	rand.Read(hash[:])
	outpoint := &wire.OutPoint{
		Hash:  hash,
		Index: 12345,
	}

	b.ResetTimer()

	b.Run("Encode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = encodeOrderedOutPoint(outpoint)
		}
	})

	encoded := encodeOrderedOutPoint(outpoint)
	b.Run("Decode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = decodeOrderedOutPoint(encoded)
		}
	})
}

// BenchmarkExpiryCompositeKeyEncoding benchmarks composite key encoding/decoding.
func BenchmarkExpiryCompositeKeyEncoding(b *testing.B) {
	hash := chainhash.Hash{}
	rand.Read(hash[:])
	outpoint := &wire.OutPoint{
		Hash:  hash,
		Index: 12345,
	}
	expiryKey := uint64(1000000)

	b.ResetTimer()

	b.Run("Encode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = encodeExpiryOutpointCompositeKey(expiryKey, outpoint)
		}
	})

	encoded := encodeExpiryOutpointCompositeKey(expiryKey, outpoint)
	b.Run("Decode", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _, _ = decodeExpiryOutpointCompositeKey(encoded)
		}
	})
}

// BenchmarkConnectBlock benchmarks block connection performance
func BenchmarkConnectBlock(b *testing.B) {
	db, teardown, err := createBenchDB()
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		b.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	// Create and initialize index
	err = db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	})
	if err != nil {
		b.Fatalf("Failed to create index: %v", err)
	}

	err = idx.Init()
	if err != nil {
		b.Fatalf("Failed to initialize index: %v", err)
	}

	// Create test blocks with varying transaction counts
	blocks := []*btcutil.Block{
		createBenchBlock(b, 150, 1),   // 1 tx
		createBenchBlock(b, 151, 10),  // 10 txs
		createBenchBlock(b, 152, 100), // 100 txs
		createBenchBlock(b, 153, 500), // 500 txs
	}

	for _, block := range blocks {
		txCount := len(block.Transactions())
		b.Run("TxCount", func(b *testing.B) {
			b.ReportMetric(float64(txCount), "txs/block")

			for i := 0; i < b.N; i++ {
				err = db.Update(func(dbTx database.Tx) error {
					spentTxOuts := []blockchain.SpentTxOut{}
					return idx.ConnectBlock(dbTx, block, spentTxOuts)
				})
				if err != nil {
					b.Fatalf("Failed to connect block: %v", err)
				}
			}
		})
	}
}

// BenchmarkScanExpiringUTXOs benchmarks UTXO scanning performance
func BenchmarkScanExpiringUTXOs(b *testing.B) {
	db, teardown, err := createBenchDB()
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		b.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	// Create and initialize index
	err = db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	})
	if err != nil {
		b.Fatalf("Failed to create index: %v", err)
	}

	err = idx.Init()
	if err != nil {
		b.Fatalf("Failed to initialize index: %v", err)
	}

	// Add some test data by connecting blocks
	for height := int32(150); height < 160; height++ {
		block := createBenchBlock(b, height, 100) // 100 txs per block
		err = db.Update(func(dbTx database.Tx) error {
			spentTxOuts := []blockchain.SpentTxOut{}
			return idx.ConnectBlock(dbTx, block, spentTxOuts)
		})
		if err != nil {
			b.Fatalf("Failed to connect block %d: %v", height, err)
		}
	}

	// Benchmark different scan sizes
	scanSizes := []int{10, 100, 1000, 10000}

	for _, maxResults := range scanSizes {
		b.Run("MaxResults", func(b *testing.B) {
			b.ReportMetric(float64(maxResults), "max_results")

			for i := 0; i < b.N; i++ {
				_, _, err := idx.ScanExpiringUTXOs(0, 1000000, maxResults, nil)
				if err != nil {
					b.Fatalf("Failed to scan UTXOs: %v", err)
				}
			}
		})
	}
}

// BenchmarkGetStats benchmarks statistics gathering
func BenchmarkGetStats(b *testing.B) {
	db, teardown, err := createBenchDB()
	if err != nil {
		b.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		b.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	// Create and initialize index
	err = db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	})
	if err != nil {
		b.Fatalf("Failed to create index: %v", err)
	}

	err = idx.Init()
	if err != nil {
		b.Fatalf("Failed to initialize index: %v", err)
	}

	// Add some test data
	for height := int32(150); height < 155; height++ {
		block := createBenchBlock(b, height, 50)
		err = db.Update(func(dbTx database.Tx) error {
			spentTxOuts := []blockchain.SpentTxOut{}
			return idx.ConnectBlock(dbTx, block, spentTxOuts)
		})
		if err != nil {
			b.Fatalf("Failed to connect block %d: %v", height, err)
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := idx.GetStats()
		if err != nil {
			b.Fatalf("Failed to get stats: %v", err)
		}
	}
}

// BenchmarkGetStatsScaling compares the constant-time RPC path with the
// explicitly requested full audit as the index grows.
func BenchmarkGetStatsScaling(b *testing.B) {
	for _, total := range []int{1, 1000, 100000} {
		b.Run(fmt.Sprintf("UTXOs_%d", total), func(b *testing.B) {
			db, teardown, err := createBenchDB()
			if err != nil {
				b.Fatalf("create database: %v", err)
			}
			defer teardown()
			idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
			if err != nil {
				b.Fatalf("create index: %v", err)
			}
			if err := db.Update(func(dbTx database.Tx) error {
				return idx.Create(dbTx)
			}); err != nil {
				b.Fatalf("initialize index: %v", err)
			}

			const batchSize = 1000
			for start := 0; start < total; start += batchSize {
				end := start + batchSize
				if end > total {
					end = total
				}
				if err := db.Update(func(dbTx database.Tx) error {
					for i := start; i < end; i++ {
						op := wire.OutPoint{
							Hash:  chainhash.DoubleHashH([]byte(fmt.Sprintf("stats-bench-%d", i))),
							Index: uint32(i),
						}
						if err := putTxOutMappingWithoutStats(
							dbTx, &op, uint64(i%100), int64(i+1),
						); err != nil {

							return err
						}
					}
					return nil
				}); err != nil {
					b.Fatalf("seed index: %v", err)
				}
			}
			distinct := total
			if distinct > 100 {
				distinct = 100
			}
			if err := db.Update(func(dbTx database.Tx) error {
				return dbPutIndexStats(dbTx, persistedIndexStats{
					totalUTXOs:      uint64(total),
					totalExpiryKeys: uint64(distinct),
				})
			}); err != nil {
				b.Fatalf("persist stats: %v", err)
			}

			b.Run("Persisted", func(b *testing.B) {
				b.ReportMetric(float64(total), "indexed_utxos")
				for i := 0; i < b.N; i++ {
					if _, err := idx.GetStats(); err != nil {
						b.Fatalf("get stats: %v", err)
					}
				}
			})
			b.Run("FullAudit", func(b *testing.B) {
				b.ReportMetric(float64(total), "indexed_utxos")
				for i := 0; i < b.N; i++ {
					if _, err := idx.AuditStats(); err != nil {
						b.Fatalf("audit stats: %v", err)
					}
				}
			})
		})
	}
}

// createBenchBlock creates a test block for benchmarking with specified transaction count
func createBenchBlock(b *testing.B, height int32, txCount int) *btcutil.Block {
	// Create a block header
	prevHash := chainhash.Hash{}
	rand.Read(prevHash[:])
	merkleRoot := chainhash.Hash{}
	rand.Read(merkleRoot[:])

	header := &wire.BlockHeader{
		Version:    1,
		PrevBlock:  prevHash,
		MerkleRoot: merkleRoot,
		Timestamp:  time.Now(),
		Bits:       0x207fffff,
		Nonce:      0,
	}

	// Create transactions
	var transactions []*wire.MsgTx

	// Coinbase transaction (always first)
	coinbaseTx := &wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{
			{
				PreviousOutPoint: wire.OutPoint{Index: 0xffffffff},
				SignatureScript:  []byte{0x00, 0x00},
				Sequence:         0xffffffff,
			},
		},
		TxOut: []*wire.TxOut{
			{
				Value:    5000000000,   // 50 BTC
				PkScript: []byte{0x51}, // OP_1
			},
		},
		LockTime: 0,
	}
	transactions = append(transactions, coinbaseTx)

	// Add regular transactions
	for i := 1; i < txCount; i++ {
		// Create random input
		inputHash := chainhash.Hash{}
		rand.Read(inputHash[:])

		tx := &wire.MsgTx{
			Version: 1,
			TxIn: []*wire.TxIn{
				{
					PreviousOutPoint: wire.OutPoint{
						Hash:  inputHash,
						Index: uint32(i % 10),
					},
					SignatureScript: []byte{0x47, 0x30, 0x44}, // Mock signature
					Sequence:        0xffffffff,
				},
			},
			TxOut: []*wire.TxOut{
				{
					Value:    1000000,                  // 0.01 BTC
					PkScript: []byte{0x76, 0xa9, 0x14}, // Mock P2PKH
				},
				{
					Value:    999000,                   // Change output
					PkScript: []byte{0x76, 0xa9, 0x14}, // Mock P2PKH
				},
			},
			LockTime: 0,
		}
		transactions = append(transactions, tx)
	}

	// Create block
	msgBlock := &wire.MsgBlock{
		Header:       *header,
		Transactions: transactions,
	}

	block := btcutil.NewBlock(msgBlock)
	block.SetHeight(height)

	return block
}

// createBenchDB creates a temporary database for benchmarking
func createBenchDB() (database.DB, func(), error) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "expiryindex_bench_")
	if err != nil {
		return nil, nil, err
	}

	dbPath := filepath.Join(tmpDir, "bench.db")

	// Create database
	db, err := database.Create("ffldb", dbPath, wire.TestNet3)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, nil, err
	}

	teardown := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return db, teardown, nil
}
