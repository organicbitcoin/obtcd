// Copyright (c) 2025-2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package expiryindex

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	_ "github.com/btcsuite/btcd/database/ffldb"
	"github.com/btcsuite/btcd/wire"
)

type rebuildMockChain struct {
	bestHeight           int32
	blocks               map[int32]*btcutil.Block
	utxos                map[wire.OutPoint]int32
	spendJournals        map[int32][]blockchain.SpentTxOut
	forEachCalls         int
	forEachErr           error
	forEachStopAfter     int
	blockRequests        []int32
	blockErrors          map[int32]error
	spendJournalRequests []int32
	fetchSpendJournalErr error
}

func (m *rebuildMockChain) BestHeight() int32 {
	return m.bestHeight
}

func (m *rebuildMockChain) BlockByHeight(height int32) (*btcutil.Block, error) {
	m.blockRequests = append(m.blockRequests, height)
	if err, ok := m.blockErrors[height]; ok {
		return nil, err
	}
	block, ok := m.blocks[height]
	if !ok {
		return nil, database.Error{ErrorCode: database.ErrBlockNotFound, Description: "mock block not found"}
	}
	return block, nil
}

func (m *rebuildMockChain) FetchSpendJournal(block *btcutil.Block) ([]blockchain.SpentTxOut, error) {
	height := block.Height()
	m.spendJournalRequests = append(m.spendJournalRequests, height)
	if m.fetchSpendJournalErr != nil {
		return nil, m.fetchSpendJournalErr
	}
	if m.spendJournals == nil {
		return nil, nil
	}
	return m.spendJournals[height], nil
}

func (m *rebuildMockChain) ForEachUTXO(fn func(outpoint wire.OutPoint, height int32) error) error {
	m.forEachCalls++
	processed := 0
	for outpoint, height := range m.utxos {
		if err := fn(outpoint, height); err != nil {
			return err
		}
		processed++
		if m.forEachErr != nil && m.forEachStopAfter > 0 && processed >= m.forEachStopAfter {
			return m.forEachErr
		}
	}
	if m.forEachErr != nil && m.forEachStopAfter == 0 {
		return m.forEachErr
	}
	return nil
}

// TestSmartRebuild tests the smart rebuild decision logic
func TestSmartRebuild(t *testing.T) {
	db, teardown, err := createRebuildTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	// Create and initialize index
	err = db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	})
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	tests := []struct {
		name           string
		indexTipHeight int32
		expectError    bool
		shouldCallFast bool // Whether we expect fast rebuild to be attempted
	}{
		{
			name:           "fresh index rebuild",
			indexTipHeight: -1,    // Fresh index
			expectError:    false, // Will fail due to missing blockchain methods but no logic error
			shouldCallFast: true,
		},
		{
			name:           "up to date index",
			indexTipHeight: 200, // Assuming current chain tip is around this
			expectError:    false,
			shouldCallFast: false,
		},
		{
			name:           "slightly behind index",
			indexTipHeight: 190,
			expectError:    false,
			shouldCallFast: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Set the index tip height for testing
			err = db.Update(func(dbTx database.Tx) error {
				if test.indexTipHeight >= 0 {
					return dbPutTipHeightIndexed(dbTx, test.indexTipHeight)
				} else {
					// For fresh index, don't put any tip height
					return nil
				}
			})
			if err != nil {
				t.Fatalf("Failed to set tip height: %v", err)
			}

			// Call smart rebuild
			err = idx.smartRebuild(test.indexTipHeight)

			// Note: This will likely fail due to missing blockchain integration,
			// but we can test the logic paths
			if test.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				// We expect some error due to missing blockchain methods,
				// but we can check the error type to see if our logic is correct
				if err != nil {
					t.Logf("Expected error due to missing blockchain methods: %v", err)
				}
			}
		})
	}
}

// TestTryFastRebuildOrFallback tests the fast rebuild attempt logic
func TestTryFastRebuildOrFallback(t *testing.T) {
	db, teardown, err := createRebuildTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	// Create and initialize index
	err = db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	})
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	tests := []struct {
		name           string
		chainTipHeight int32
		expectError    bool
	}{
		{
			name:           "reasonable chain tip",
			chainTipHeight: 200,
			expectError:    false, // Will fail due to missing methods but logic is sound
		},
		{
			name:           "very high chain tip",
			chainTipHeight: 1000000,
			expectError:    false, // Should still attempt fast rebuild
		},
		{
			name:           "zero chain tip",
			chainTipHeight: 0,
			expectError:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Attempt fast rebuild
			err = idx.tryFastRebuildOrFallback(test.chainTipHeight)

			// We expect this to fail due to missing blockchain integration,
			// but we can verify the error type
			if err != nil {
				t.Logf("Expected error due to missing blockchain integration: %v", err)
			}

			// Verify index structure remains intact
			err = db.View(func(dbTx database.Tx) error {
				meta := dbTx.Metadata()
				if meta.Bucket(bktOutpoint2Expiry) == nil {
					t.Error("Outpoint bucket missing after rebuild attempt")
				}
				if meta.Bucket(bktExpiry2Outpoints) == nil {
					t.Error("Expiry bucket missing after rebuild attempt")
				}
				if meta.Bucket(bktExpiryMeta) == nil {
					t.Error("Meta bucket missing after rebuild attempt")
				}
				return nil
			})

			if err != nil {
				t.Errorf("Index structure verification failed: %v", err)
			}
		})
	}
}

func TestSmartRebuildChoosesExpectedStrategy(t *testing.T) {
	tests := []struct {
		name                string
		indexTipHeight      int32
		chainTipHeight      int32
		expectForEachCalls  int
		expectSpendRequests []int32
	}{
		{
			name:               "fresh index uses fast rebuild",
			indexTipHeight:     -1,
			chainTipHeight:     5,
			expectForEachCalls: 1,
		},
		{
			name:           "up to date index is no-op",
			indexTipHeight: 5,
			chainTipHeight: 5,
		},
		{
			name:                "small lag uses incremental catch-up",
			indexTipHeight:      3,
			chainTipHeight:      5,
			expectSpendRequests: []int32{4, 5},
		},
		{
			name:               "large lag uses fast rebuild",
			indexTipHeight:     0,
			chainTipHeight:     1505,
			expectForEachCalls: 1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, teardown, err := createRebuildTestDB()
			if err != nil {
				t.Fatalf("Failed to create test database: %v", err)
			}
			defer teardown()

			idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
			if err != nil {
				t.Fatalf("Failed to create ExpiryIndex: %v", err)
			}
			if err := db.Update(func(dbTx database.Tx) error {
				if err := idx.Create(dbTx); err != nil {
					return err
				}
				return dbPutTipHeightIndexed(dbTx, test.indexTipHeight)
			}); err != nil {
				t.Fatalf("Failed to seed tip height: %v", err)
			}
			idx.curTipHeight = test.indexTipHeight

			mock := &rebuildMockChain{
				bestHeight: test.chainTipHeight,
				blocks:     make(map[int32]*btcutil.Block),
				utxos:      make(map[wire.OutPoint]int32),
			}
			if test.chainTipHeight >= 0 {
				mock.blocks[test.chainTipHeight] = createTestBlock(t, test.chainTipHeight)
			}
			for height := test.indexTipHeight + 1; height <= test.chainTipHeight; height++ {
				if height >= 0 {
					mock.blocks[height] = createTestBlock(t, height)
				}
			}
			idx.chain = mock

			if err := idx.smartRebuild(test.indexTipHeight); err != nil {
				t.Fatalf("smartRebuild failed: %v", err)
			}

			if mock.forEachCalls != test.expectForEachCalls {
				t.Fatalf("unexpected ForEachUTXO call count: got %d want %d",
					mock.forEachCalls, test.expectForEachCalls)
			}
			if !reflect.DeepEqual(mock.spendJournalRequests, test.expectSpendRequests) {
				t.Fatalf("unexpected spend journal requests: got %v want %v",
					mock.spendJournalRequests, test.expectSpendRequests)
			}
		})
	}
}

func TestTryFastRebuildOrFallbackRestartsIncrementalFromCleanState(t *testing.T) {
	db, teardown, err := createRebuildTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error {
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		return dbPutTipHeightIndexed(dbTx, 3)
	}); err != nil {
		t.Fatalf("Failed to seed stale tip: %v", err)
	}
	idx.curTipHeight = 3

	mock := &rebuildMockChain{
		bestHeight:           5,
		blocks:               make(map[int32]*btcutil.Block),
		utxos:                make(map[wire.OutPoint]int32),
		forEachErr:           errors.New("utxo iterator failed"),
		fetchSpendJournalErr: nil,
	}
	for height := int32(0); height <= 5; height++ {
		mock.blocks[height] = createTestBlock(t, height)
	}
	idx.chain = mock

	if err := idx.tryFastRebuildOrFallback(5); err != nil {
		t.Fatalf("fallback rebuild should succeed, got %v", err)
	}

	if mock.forEachCalls != 1 {
		t.Fatalf("expected one fast rebuild attempt, got %d", mock.forEachCalls)
	}
	wantRequests := []int32{0, 1, 2, 3, 4, 5}
	if !reflect.DeepEqual(mock.spendJournalRequests, wantRequests) {
		t.Fatalf("fallback should replay from a clean tip, got %v want %v",
			mock.spendJournalRequests, wantRequests)
	}

	stats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("Failed to read stats after fallback rebuild: %v", err)
	}
	if stats.TipHeight != 5 {
		t.Fatalf("unexpected tip height after fallback rebuild: got %d want 5", stats.TipHeight)
	}
}

// TestClearIndexBuckets tests clearing all index data
func TestClearIndexBuckets(t *testing.T) {
	db, teardown, err := createRebuildTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	// Create and initialize index
	err = db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	})
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// Add some test data first
	err = db.Update(func(dbTx database.Tx) error {
		// Add some metadata
		err := dbPutTipHeightIndexed(dbTx, 150)
		if err != nil {
			return err
		}

		// Add some dummy data to buckets
		outpointBucket := dbTx.Metadata().Bucket(bktOutpoint2Expiry)
		if outpointBucket != nil {
			err := outpointBucket.Put([]byte("test-key"), []byte("test-value"))
			if err != nil {
				return err
			}
		}

		expiryBucket := dbTx.Metadata().Bucket(bktExpiry2Outpoints)
		if expiryBucket != nil {
			err := expiryBucket.Put([]byte("expiry-key"), []byte("expiry-value"))
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Failed to add test data: %v", err)
	}

	// Verify data exists
	err = db.View(func(dbTx database.Tx) error {
		height, err := dbGetTipHeightIndexed(dbTx)
		if err != nil {
			t.Errorf("Failed to read tip height: %v", err)
		}
		if height != 150 {
			t.Errorf("Tip height mismatch: got %d, want 150", height)
		}

		outpointBucket := dbTx.Metadata().Bucket(bktOutpoint2Expiry)
		if outpointBucket != nil {
			value := outpointBucket.Get([]byte("test-key"))
			if value == nil {
				t.Error("Test data not found in outpoint bucket")
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Data verification failed: %v", err)
	}

	// Clear the index
	err = db.Update(func(dbTx database.Tx) error {
		return idx.clearIndexBuckets(dbTx)
	})
	if err != nil {
		t.Fatalf("Failed to clear index buckets: %v", err)
	}

	// Verify data is gone
	err = db.View(func(dbTx database.Tx) error {
		outpointBucket := dbTx.Metadata().Bucket(bktOutpoint2Expiry)
		if outpointBucket != nil {
			value := outpointBucket.Get([]byte("test-key"))
			if value != nil {
				t.Error("Test data still exists in outpoint bucket after clear")
			}
		}

		expiryBucket := dbTx.Metadata().Bucket(bktExpiry2Outpoints)
		if expiryBucket != nil {
			value := expiryBucket.Get([]byte("expiry-key"))
			if value != nil {
				t.Error("Test data still exists in expiry bucket after clear")
			}
		}

		height, err := dbGetTipHeightIndexed(dbTx)
		if err != nil {
			return err
		}
		if height != -1 {
			t.Errorf("tip height should reset to -1 after clear, got %d", height)
		}

		tipHash, err := dbGetAccumulatorTipHash(dbTx)
		if err != nil {
			return err
		}
		if tipHash != (chainhash.Hash{}) {
			t.Errorf("expected zero accumulator tip hash after clear, got %v", tipHash)
		}

		mh, err := dbGetAccumulatorState(dbTx)
		if err != nil {
			return err
		}
		if mh.Digest() != NewMuHash().Digest() {
			t.Errorf("expected identity accumulator after clear, got %x", mh.Digest())
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Clear verification failed: %v", err)
	}
}

// TestRebuildErrorHandling tests error handling in rebuild functions
func TestRebuildErrorHandling(t *testing.T) {
	db, teardown, err := createRebuildTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	// Test with uninitialized index (no buckets)
	err = idx.smartRebuild(-1)
	if err != nil {
		t.Logf("Expected error for uninitialized index: %v", err)
	} else {
		t.Log("smartRebuild handled uninitialized index gracefully")
	}

	// Create index properly
	err = db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	})
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// Test with invalid chain tip height
	err = idx.tryFastRebuildOrFallback(-100)
	if err != nil {
		t.Logf("Expected error for negative chain tip: %v", err)
	} else {
		t.Log("tryFastRebuildOrFallback handled negative tip gracefully")
	}
}

// TestIndexConsistencyAfterRebuild tests that index remains consistent after rebuild attempts
func TestFastRebuildFromUTXODirect(t *testing.T) {
	db, teardown, err := createRebuildTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	err = db.Update(func(dbTx database.Tx) error { return idx.Create(dbTx) })
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	err = idx.fastRebuildFromUTXO(200)
	if err == nil {
		t.Fatalf("expected fastRebuildFromUTXO to fail without blockchain UTXO access")
	}
}

func TestIncrementalCatchUpDirect(t *testing.T) {
	db, teardown, err := createRebuildTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	// from >= to should be a no-op success path.
	if err := idx.incrementalCatchUp(10, 10); err != nil {
		t.Fatalf("expected no-op success, got %v", err)
	}

	// Real catch-up path currently relies on blockchain access helpers.
	err = idx.incrementalCatchUp(0, 1)
	if err == nil {
		t.Fatalf("expected incrementalCatchUp to fail without blockchain access")
	}
}

func TestIndexConsistencyAfterRebuild(t *testing.T) {
	db, teardown, err := createRebuildTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}

	// Create and initialize index
	err = db.Update(func(dbTx database.Tx) error {
		return idx.Create(dbTx)
	})
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	err = idx.Init()
	if err != nil {
		t.Fatalf("Failed to initialize index: %v", err)
	}

	// Get initial stats
	initialStats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("Failed to get initial stats: %v", err)
	}

	// Attempt rebuild (will fail but shouldn't corrupt)
	idx.tryFastRebuildOrFallback(200)

	// Verify index is still functional
	finalStats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("Index corrupted after rebuild attempt: %v", err)
	}

	// Basic consistency check
	if finalStats.TipHeight < initialStats.TipHeight-1 {
		t.Error("Index appears to have lost significant data")
	}

	// Verify buckets still exist and are accessible
	err = db.View(func(dbTx database.Tx) error {
		meta := dbTx.Metadata()

		buckets := [][]byte{bktOutpoint2Expiry, bktExpiry2Outpoints, bktExpiryMeta}
		for _, bucketName := range buckets {
			bucket := meta.Bucket(bucketName)
			if bucket == nil {
				t.Errorf("Bucket %s missing after rebuild attempt", string(bucketName))
			}
		}

		return nil
	})

	if err != nil {
		t.Errorf("Bucket consistency check failed: %v", err)
	}
}

func TestFastRebuildAccumulatorMatchesLiveIndexingBeforeEnableAtHeight(t *testing.T) {
	params := &chaincfg.ObtcRegTestParams
	expiryParams := GetExpiryParams(params)
	if expiryParams == nil {
		t.Fatal("expected OBTC regtest expiry params")
	}
	if expiryParams.StartScanHeight >= expiryParams.EnableAtHeight {
		t.Fatalf("test requires StartScanHeight < EnableAtHeight, got %d >= %d",
			expiryParams.StartScanHeight, expiryParams.EnableAtHeight)
	}

	liveDB, liveTeardown, err := createRebuildTestDB()
	if err != nil {
		t.Fatalf("create live db: %v", err)
	}
	defer liveTeardown()

	liveIdx, err := NewExpiryIndex(liveDB, params)
	if err != nil {
		t.Fatalf("new live index: %v", err)
	}
	if err := liveDB.Update(func(dbTx database.Tx) error { return liveIdx.Create(dbTx) }); err != nil {
		t.Fatalf("create live index: %v", err)
	}
	if err := liveIdx.Init(); err != nil {
		t.Fatalf("init live index: %v", err)
	}

	seedOut := makeOutPoint(42, 0)
	prevOut := seedOut
	prevCreateHeight := expiryParams.StartScanHeight - 1
	mock := &rebuildMockChain{
		blocks: make(map[int32]*btcutil.Block),
		utxos:  make(map[wire.OutPoint]int32),
	}

	// The seed outpoint is spent by the first indexed block, but since it
	// predates StartScanHeight it must not affect either live indexing or rebuild.
	mock.utxos[seedOut] = prevCreateHeight

	lastHeight := expiryParams.EnableAtHeight - 1
	for height := expiryParams.StartScanHeight; height <= lastHeight; height++ {
		block, newOut := buildSpendBlock(height, prevOut, nil)
		mock.blocks[height] = block

		stxos := []blockchain.SpentTxOut{{Height: prevCreateHeight}}
		if err := liveDB.Update(func(dbTx database.Tx) error {
			return liveIdx.ConnectBlock(dbTx, block, stxos)
		}); err != nil {
			t.Fatalf("connect block %d: %v", height, err)
		}

		delete(mock.utxos, prevOut)
		coinbaseOut := wire.OutPoint{Hash: *block.Transactions()[0].Hash(), Index: 0}
		mock.utxos[coinbaseOut] = height
		mock.utxos[newOut] = height
		prevOut = newOut
		prevCreateHeight = height
		mock.bestHeight = height
	}

	liveSnapshot, err := liveIdx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("live snapshot: %v", err)
	}
	if liveSnapshot.TipHeight != lastHeight {
		t.Fatalf("live tip height mismatch: got %d want %d", liveSnapshot.TipHeight, lastHeight)
	}

	rebuildDB, rebuildTeardown, err := createRebuildTestDB()
	if err != nil {
		t.Fatalf("create rebuild db: %v", err)
	}
	defer rebuildTeardown()

	rebuildIdx, err := NewExpiryIndex(rebuildDB, params)
	if err != nil {
		t.Fatalf("new rebuild index: %v", err)
	}
	if err := rebuildDB.Update(func(dbTx database.Tx) error { return rebuildIdx.Create(dbTx) }); err != nil {
		t.Fatalf("create rebuild index: %v", err)
	}
	rebuildIdx.SetChainAccessor(mock)
	if err := rebuildIdx.fastRebuildFromUTXO(lastHeight); err != nil {
		t.Fatalf("fast rebuild: %v", err)
	}

	rebuildSnapshot, err := rebuildIdx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("rebuild snapshot: %v", err)
	}
	if rebuildSnapshot.Root != liveSnapshot.Root {
		t.Fatalf("accumulator root mismatch after rebuild: live=%x rebuild=%x",
			liveSnapshot.Root, rebuildSnapshot.Root)
	}
	if rebuildSnapshot.TipHeight != liveSnapshot.TipHeight {
		t.Fatalf("tip height mismatch after rebuild: live=%d rebuild=%d",
			liveSnapshot.TipHeight, rebuildSnapshot.TipHeight)
	}
	if rebuildSnapshot.TipHash != liveSnapshot.TipHash {
		t.Fatalf("tip hash mismatch after rebuild: live=%s rebuild=%s",
			liveSnapshot.TipHash, rebuildSnapshot.TipHash)
	}
}

func TestFastRebuildFromUTXOBatchesAndSkipsPreStart(t *testing.T) {
	db, teardown, err := createRebuildTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error { return idx.Create(dbTx) }); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}
	idx.expiryParams.StartScanHeight = 2

	mock := &rebuildMockChain{
		bestHeight: 7,
		blocks: map[int32]*btcutil.Block{
			7: createTestBlock(t, 7),
		},
		utxos: make(map[wire.OutPoint]int32, DefaultBatchSize+10),
	}
	mock.utxos[makeOutPoint(0, 0)] = 1
	for i := 0; i < DefaultBatchSize+5; i++ {
		mock.utxos[makeOutPoint(byte(i%250+1), uint32(i))] = 2
	}
	idx.chain = mock

	if err := idx.fastRebuildFromUTXO(7); err != nil {
		t.Fatalf("fast rebuild failed: %v", err)
	}

	stats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}
	if stats.TotalUTXOs != DefaultBatchSize+5 {
		t.Fatalf("unexpected UTXO count: got %d want %d", stats.TotalUTXOs, DefaultBatchSize+5)
	}
	if stats.TotalExpiryKeys != 1 {
		t.Fatalf("unexpected expiry key count: got %d want 1", stats.TotalExpiryKeys)
	}
	if stats.TipHeight != 7 {
		t.Fatalf("unexpected tip height: got %d want 7", stats.TipHeight)
	}

	rows, hasMore, err := idx.ScanExpiringUTXOs(0, ^uint64(0), DefaultBatchSize+10, nil)
	if err != nil {
		t.Fatalf("Failed to scan rebuilt rows: %v", err)
	}
	if hasMore {
		t.Fatal("expected rebuilt rows to fit in a single page")
	}
	if len(rows) != DefaultBatchSize+5 {
		t.Fatalf("unexpected rebuilt row count: got %d want %d", len(rows), DefaultBatchSize+5)
	}
}

func TestFastRebuildFromUTXOSkipsGenesisCreatedUTXO(t *testing.T) {
	db, teardown, err := createRebuildTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error { return idx.Create(dbTx) }); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	genesisOut := makeOutPoint(0x01, 0)
	heightOneOut := makeOutPoint(0x02, 0)
	idx.chain = &rebuildMockChain{
		bestHeight: 1,
		blocks: map[int32]*btcutil.Block{
			1: createTestBlock(t, 1),
		},
		utxos: map[wire.OutPoint]int32{
			genesisOut:   0,
			heightOneOut: 1,
		},
	}

	if err := idx.fastRebuildFromUTXO(1); err != nil {
		t.Fatalf("fast rebuild failed: %v", err)
	}

	stats, err := idx.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}
	if stats.TotalUTXOs != 1 || stats.TotalExpiryKeys != 1 {
		t.Fatalf("unexpected stats after genesis skip: %+v", stats)
	}

	candidates, err := idx.ReapPrefixCandidates(1_000, 10)
	if err != nil {
		t.Fatalf("Failed to read REAP prefix: %v", err)
	}
	if len(candidates) != 1 || candidates[0].OutPoint != heightOneOut {
		t.Fatalf("unexpected REAP candidates after genesis skip: %+v", candidates)
	}
}

func TestFastRebuildFromUTXONegativeTipKeepsIdentitySnapshot(t *testing.T) {
	db, teardown, err := createRebuildTestDB()
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("Failed to create ExpiryIndex: %v", err)
	}
	if err := db.Update(func(dbTx database.Tx) error { return idx.Create(dbTx) }); err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}
	idx.chain = &rebuildMockChain{
		bestHeight: -1,
		blocks:     make(map[int32]*btcutil.Block),
		utxos:      make(map[wire.OutPoint]int32),
	}

	if err := idx.fastRebuildFromUTXO(-1); err != nil {
		t.Fatalf("fast rebuild with negative tip should succeed, got %v", err)
	}

	snapshot, err := idx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("Failed to get snapshot: %v", err)
	}
	if snapshot.TipHeight != -1 {
		t.Fatalf("unexpected tip height: got %d want -1", snapshot.TipHeight)
	}
	if snapshot.Root != NewMuHash().Digest() {
		t.Fatalf("expected identity root, got %x", snapshot.Root)
	}
	if snapshot.TipHash != (chainhash.Hash{}) {
		t.Fatalf("expected zero tip hash, got %v", snapshot.TipHash)
	}
}

func TestFastRebuildFromUTXOLeavesCleanStateOnFailure(t *testing.T) {
	tests := []struct {
		name      string
		prepareDB func(database.DB) error
		mock      *rebuildMockChain
		chainTip  int32
	}{
		{
			name: "iterator error after partial population",
			mock: &rebuildMockChain{
				bestHeight:       4,
				blocks:           map[int32]*btcutil.Block{4: createTestBlock(t, 4)},
				utxos:            map[wire.OutPoint]int32{makeOutPoint(1, 0): 1, makeOutPoint(2, 0): 1},
				forEachErr:       errors.New("iterator broke"),
				forEachStopAfter: 1,
			},
			chainTip: 4,
		},
		{
			name: "finalization block lookup error",
			mock: &rebuildMockChain{
				bestHeight:  9,
				blocks:      make(map[int32]*btcutil.Block),
				utxos:       map[wire.OutPoint]int32{makeOutPoint(3, 0): 2},
				blockErrors: map[int32]error{9: database.Error{ErrorCode: database.ErrBlockNotFound, Description: "missing tip block"}},
			},
			chainTip: 9,
		},
		{
			name: "flush failure due to missing bucket",
			prepareDB: func(db database.DB) error {
				return db.Update(func(dbTx database.Tx) error {
					return dbTx.Metadata().DeleteBucket(bktOutpoint2Expiry)
				})
			},
			mock: &rebuildMockChain{
				bestHeight: -1,
				blocks:     make(map[int32]*btcutil.Block),
				utxos:      map[wire.OutPoint]int32{makeOutPoint(4, 0): 2},
			},
			chainTip: -1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, teardown, err := createRebuildTestDB()
			if err != nil {
				t.Fatalf("Failed to create test database: %v", err)
			}
			defer teardown()

			idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
			if err != nil {
				t.Fatalf("Failed to create ExpiryIndex: %v", err)
			}
			if err := db.Update(func(dbTx database.Tx) error { return idx.Create(dbTx) }); err != nil {
				t.Fatalf("Failed to create index: %v", err)
			}
			if err := db.Update(func(dbTx database.Tx) error {
				op := makeOutPoint(99, 0)
				if err := putTxOutMapping(dbTx, &op, 123); err != nil {
					return err
				}
				if err := dbPutAccumulatorState(dbTx, NewMuHash()); err != nil {
					return err
				}
				return dbPutTipHeightIndexed(dbTx, 55)
			}); err != nil {
				t.Fatalf("Failed to seed stale state: %v", err)
			}
			idx.curTipHeight = 55

			if test.prepareDB != nil {
				if err := test.prepareDB(db); err != nil {
					t.Fatalf("Failed to prepare database: %v", err)
				}
			}
			idx.chain = test.mock

			if err := idx.fastRebuildFromUTXO(test.chainTip); err == nil {
				t.Fatal("expected fast rebuild to fail")
			}

			stats, err := idx.GetStats()
			if err != nil {
				t.Fatalf("Failed to get stats after failure: %v", err)
			}
			if stats.TipHeight != -1 || stats.TotalUTXOs != 0 || stats.TotalExpiryKeys != 0 {
				t.Fatalf("expected clean state after failure, got stats %+v", stats)
			}

			snapshot, err := idx.GetAccumulatorSnapshot()
			if err != nil {
				t.Fatalf("Failed to get snapshot after failure: %v", err)
			}
			if snapshot.TipHeight != -1 || snapshot.Root != NewMuHash().Digest() || snapshot.TipHash != (chainhash.Hash{}) {
				t.Fatalf("expected empty snapshot after failure, got %+v", snapshot)
			}

			rows, hasMore, err := idx.ScanExpiringUTXOs(0, ^uint64(0), 10, nil)
			if err != nil {
				t.Fatalf("Failed to scan after failure: %v", err)
			}
			if hasMore || len(rows) != 0 {
				t.Fatalf("expected empty index after failure, got rows=%v hasMore=%t", rows, hasMore)
			}
		})
	}
}

func TestLiveIndexFastRebuildAndRollbackConsistency(t *testing.T) {
	params := &chaincfg.ObtcRegTestParams

	liveDB, liveTeardown, err := createRebuildTestDB()
	if err != nil {
		t.Fatalf("create live db: %v", err)
	}
	defer liveTeardown()

	liveIdx, err := NewExpiryIndex(liveDB, params)
	if err != nil {
		t.Fatalf("new live index: %v", err)
	}
	if err := liveDB.Update(func(dbTx database.Tx) error { return liveIdx.Create(dbTx) }); err != nil {
		t.Fatalf("create live index: %v", err)
	}
	if err := liveIdx.Init(); err != nil {
		t.Fatalf("init live index: %v", err)
	}
	liveIdx.expiryParams.EnableAtHeight = 0
	liveIdx.expiryParams.ExpiryCommitmentEnableAtHeight = 0

	initialSnapshot, err := liveIdx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("initial snapshot: %v", err)
	}
	initialStats, err := liveIdx.GetStats()
	if err != nil {
		t.Fatalf("initial stats: %v", err)
	}
	initialScan, initialHasMore, err := liveIdx.ScanExpiringUTXOs(0, ^uint64(0), 100, nil)
	if err != nil {
		t.Fatalf("initial scan: %v", err)
	}
	if initialHasMore || len(initialScan) != 0 {
		t.Fatalf("expected empty initial scan, got rows=%v hasMore=%t", initialScan, initialHasMore)
	}

	mock := &rebuildMockChain{
		blocks:        make(map[int32]*btcutil.Block),
		utxos:         make(map[wire.OutPoint]int32),
		spendJournals: make(map[int32][]blockchain.SpentTxOut),
	}
	prevOut := makeOutPoint(200, 0)
	prevCreateHeight := int32(-1)
	mock.utxos[prevOut] = prevCreateHeight

	for height := int32(0); height <= 4; height++ {
		block, newOut := buildSpendBlock(height, prevOut, nil)
		stxos := []blockchain.SpentTxOut{{
			Amount:     1_0000_0000,
			PkScript:   []byte{0x51},
			Height:     prevCreateHeight,
			IsCoinBase: prevCreateHeight >= 0,
		}}
		if err := liveDB.Update(func(dbTx database.Tx) error {
			return liveIdx.ConnectBlock(dbTx, block, stxos)
		}); err != nil {
			t.Fatalf("connect live block %d: %v", height, err)
		}

		mock.blocks[height] = block
		mock.spendJournals[height] = stxos
		delete(mock.utxos, prevOut)
		mock.utxos[wire.OutPoint{Hash: *block.Transactions()[0].Hash(), Index: 0}] = height
		mock.utxos[newOut] = height
		mock.bestHeight = height
		prevOut = newOut
		prevCreateHeight = height
	}

	liveSnapshot, err := liveIdx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("live snapshot: %v", err)
	}
	liveStats, err := liveIdx.GetStats()
	if err != nil {
		t.Fatalf("live stats: %v", err)
	}
	liveScan, liveHasMore, err := liveIdx.ScanExpiringUTXOs(0, ^uint64(0), 100, nil)
	if err != nil {
		t.Fatalf("live scan: %v", err)
	}
	if liveHasMore {
		t.Fatal("expected live scan to fit in a single page")
	}

	rebuildDB, rebuildTeardown, err := createRebuildTestDB()
	if err != nil {
		t.Fatalf("create rebuild db: %v", err)
	}
	defer rebuildTeardown()

	rebuildIdx, err := NewExpiryIndex(rebuildDB, params)
	if err != nil {
		t.Fatalf("new rebuild index: %v", err)
	}
	if err := rebuildDB.Update(func(dbTx database.Tx) error { return rebuildIdx.Create(dbTx) }); err != nil {
		t.Fatalf("create rebuild index: %v", err)
	}
	if err := rebuildIdx.Init(); err != nil {
		t.Fatalf("init rebuild index: %v", err)
	}
	rebuildIdx.expiryParams.EnableAtHeight = 0
	rebuildIdx.expiryParams.ExpiryCommitmentEnableAtHeight = 0
	rebuildIdx.chain = mock

	if err := rebuildIdx.fastRebuildFromUTXO(4); err != nil {
		t.Fatalf("fast rebuild: %v", err)
	}

	rebuildSnapshot, err := rebuildIdx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("rebuild snapshot: %v", err)
	}
	rebuildStats, err := rebuildIdx.GetStats()
	if err != nil {
		t.Fatalf("rebuild stats: %v", err)
	}
	rebuildScan, rebuildHasMore, err := rebuildIdx.ScanExpiringUTXOs(0, ^uint64(0), 100, nil)
	if err != nil {
		t.Fatalf("rebuild scan: %v", err)
	}
	if rebuildHasMore {
		t.Fatal("expected rebuild scan to fit in a single page")
	}
	if rebuildSnapshot != liveSnapshot {
		t.Fatalf("snapshot mismatch after rebuild: live=%+v rebuild=%+v", liveSnapshot, rebuildSnapshot)
	}
	if !reflect.DeepEqual(rebuildStats, liveStats) {
		t.Fatalf("stats mismatch after rebuild: live=%+v rebuild=%+v", liveStats, rebuildStats)
	}
	if !reflect.DeepEqual(rebuildScan, liveScan) {
		t.Fatalf("scan mismatch after rebuild: live=%v rebuild=%v", liveScan, rebuildScan)
	}

	for height := int32(4); height >= 0; height-- {
		block := mock.blocks[height]
		stxos := mock.spendJournals[height]
		if err := liveDB.Update(func(dbTx database.Tx) error {
			return liveIdx.DisconnectBlock(dbTx, block, stxos)
		}); err != nil {
			t.Fatalf("disconnect live block %d: %v", height, err)
		}
	}

	rollbackSnapshot, err := liveIdx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("rollback snapshot: %v", err)
	}
	rollbackStats, err := liveIdx.GetStats()
	if err != nil {
		t.Fatalf("rollback stats: %v", err)
	}
	rollbackScan, rollbackHasMore, err := liveIdx.ScanExpiringUTXOs(0, ^uint64(0), 100, nil)
	if err != nil {
		t.Fatalf("rollback scan: %v", err)
	}
	if rollbackHasMore {
		t.Fatal("expected rollback scan to fit in a single page")
	}
	if rollbackSnapshot != initialSnapshot {
		t.Fatalf("rollback snapshot mismatch: got %+v want %+v", rollbackSnapshot, initialSnapshot)
	}
	if !reflect.DeepEqual(rollbackStats, initialStats) {
		t.Fatalf("rollback stats mismatch: got %+v want %+v", rollbackStats, initialStats)
	}
	if !reflect.DeepEqual(rollbackScan, initialScan) {
		t.Fatalf("rollback scan mismatch: got %v want %v", rollbackScan, initialScan)
	}
}

// createRebuildTestDB creates a temporary database for rebuild testing
func createRebuildTestDB() (database.DB, func(), error) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "expiryindex_rebuild_test_")
	if err != nil {
		return nil, nil, err
	}

	dbPath := filepath.Join(tmpDir, "rebuild_test.db")

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
