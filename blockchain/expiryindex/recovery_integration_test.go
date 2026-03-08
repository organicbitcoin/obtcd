package expiryindex

import (
	"reflect"
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/database"
	"github.com/btcsuite/btcd/wire"
)

type recoveryMockChain struct {
	bestHeight           int32
	blocks               map[int32]*btcutil.Block
	utxos                map[wire.OutPoint]int32
	spendJournals        map[int32][]blockchain.SpentTxOut
	spendJournalRequests []int32
}

func (m *recoveryMockChain) BestHeight() int32 {
	return m.bestHeight
}

func (m *recoveryMockChain) BlockByHeight(height int32) (*btcutil.Block, error) {
	block, ok := m.blocks[height]
	if !ok {
		return nil, database.Error{
			ErrorCode:   database.ErrBlockNotFound,
			Description: "recovery mock block not found",
		}
	}
	return block, nil
}

func (m *recoveryMockChain) FetchSpendJournal(block *btcutil.Block) ([]blockchain.SpentTxOut, error) {
	height := block.Height()
	m.spendJournalRequests = append(m.spendJournalRequests, height)
	return m.spendJournals[height], nil
}

func (m *recoveryMockChain) ForEachUTXO(fn func(outpoint wire.OutPoint, height int32) error) error {
	for outpoint, height := range m.utxos {
		if err := fn(outpoint, height); err != nil {
			return err
		}
	}
	return nil
}

func TestSetChainAccessorIncrementalCatchUpMatchesLiveIndex(t *testing.T) {
	params := &chaincfg.ObtcRegTestParams
	expiryParams := GetExpiryParams(params)
	if expiryParams == nil {
		t.Fatal("expected OBTC regtest expiry params")
	}

	seedTip := expiryParams.ExpiryCommitmentEnableAtHeight - 1
	chainTip := expiryParams.ExpiryCommitmentEnableAtHeight + 1

	liveDB, liveTeardown, err := createRebuildTestDB()
	if err != nil {
		t.Fatalf("create live db: %v", err)
	}
	defer liveTeardown()

	liveIdx, err := NewExpiryIndex(liveDB, params)
	if err != nil {
		t.Fatalf("new live index: %v", err)
	}
	if err := liveDB.Update(func(dbTx database.Tx) error {
		return liveIdx.Create(dbTx)
	}); err != nil {
		t.Fatalf("create live index: %v", err)
	}
	if err := liveIdx.Init(); err != nil {
		t.Fatalf("init live index: %v", err)
	}

	mock := &recoveryMockChain{
		blocks:        make(map[int32]*btcutil.Block),
		utxos:         make(map[wire.OutPoint]int32),
		spendJournals: make(map[int32][]blockchain.SpentTxOut),
	}

	prevOut := makeOutPoint(99, 0)
	prevCreateHeight := expiryParams.StartScanHeight - 1
	mock.utxos[prevOut] = prevCreateHeight

	for height := expiryParams.StartScanHeight; height <= chainTip; height++ {
		var commitRoot *[AccumulatorDigestSize]byte
		if height >= expiryParams.ExpiryCommitmentEnableAtHeight {
			snapshot, err := liveIdx.GetAccumulatorSnapshot()
			if err != nil {
				t.Fatalf("live snapshot before block %d: %v", height, err)
			}
			root := snapshot.Root
			commitRoot = &root
		}

		block, newOut := buildSpendBlock(height, prevOut, commitRoot)
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
	liveDigest, err := liveIdx.GetAccumulatorDigest()
	if err != nil {
		t.Fatalf("live digest: %v", err)
	}
	liveStats, err := liveIdx.GetStats()
	if err != nil {
		t.Fatalf("live stats: %v", err)
	}
	liveScan, liveHasMore, err := liveIdx.ScanExpiringUTXOs(0, ^uint64(0), 1000, nil)
	if err != nil {
		t.Fatalf("live scan: %v", err)
	}
	if liveHasMore {
		t.Fatal("expected live scan to fit in a single page")
	}

	recoveryDB, recoveryTeardown, err := createRebuildTestDB()
	if err != nil {
		t.Fatalf("create recovery db: %v", err)
	}
	defer recoveryTeardown()

	recoveryIdx, err := NewExpiryIndex(recoveryDB, params)
	if err != nil {
		t.Fatalf("new recovery index: %v", err)
	}
	if err := recoveryDB.Update(func(dbTx database.Tx) error {
		return recoveryIdx.Create(dbTx)
	}); err != nil {
		t.Fatalf("create recovery index: %v", err)
	}
	if err := recoveryIdx.Init(); err != nil {
		t.Fatalf("init recovery index: %v", err)
	}

	for height := expiryParams.StartScanHeight; height <= seedTip; height++ {
		block := mock.blocks[height]
		stxos := mock.spendJournals[height]
		if err := recoveryDB.Update(func(dbTx database.Tx) error {
			return recoveryIdx.ConnectBlock(dbTx, block, stxos)
		}); err != nil {
			t.Fatalf("seed recovery block %d: %v", height, err)
		}
	}

	seedStats, err := recoveryIdx.GetStats()
	if err != nil {
		t.Fatalf("seed stats: %v", err)
	}
	if seedStats.TipHeight != seedTip {
		t.Fatalf("seed tip mismatch: got %d want %d", seedStats.TipHeight, seedTip)
	}

	recoveryIdx.SetChainAccessor(mock)

	if !reflect.DeepEqual(mock.spendJournalRequests, []int32{seedTip + 1, chainTip}) {
		t.Fatalf("unexpected spend journal requests: got %v want [%d %d]",
			mock.spendJournalRequests, seedTip+1, chainTip)
	}

	recoverySnapshot, err := recoveryIdx.GetAccumulatorSnapshot()
	if err != nil {
		t.Fatalf("recovery snapshot: %v", err)
	}
	if recoverySnapshot != liveSnapshot {
		t.Fatalf("snapshot mismatch after catch-up: live=%+v recovery=%+v",
			liveSnapshot, recoverySnapshot)
	}

	recoveryDigest, err := recoveryIdx.GetAccumulatorDigest()
	if err != nil {
		t.Fatalf("recovery digest: %v", err)
	}
	if recoveryDigest != liveDigest {
		t.Fatalf("digest mismatch after catch-up: live=%x recovery=%x",
			liveDigest, recoveryDigest)
	}

	recoveryStats, err := recoveryIdx.GetStats()
	if err != nil {
		t.Fatalf("recovery stats: %v", err)
	}
	if !reflect.DeepEqual(recoveryStats, liveStats) {
		t.Fatalf("stats mismatch after catch-up: live=%+v recovery=%+v",
			liveStats, recoveryStats)
	}

	recoveryScan, recoveryHasMore, err := recoveryIdx.ScanExpiringUTXOs(0, ^uint64(0), 1000, nil)
	if err != nil {
		t.Fatalf("recovery scan: %v", err)
	}
	if recoveryHasMore {
		t.Fatal("expected recovery scan to fit in a single page")
	}
	if !reflect.DeepEqual(recoveryScan, liveScan) {
		t.Fatalf("scan mismatch after catch-up: live=%v recovery=%v",
			liveScan, recoveryScan)
	}
}
