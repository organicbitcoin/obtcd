package expiryindex

import (
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	"github.com/btcsuite/btcd/wire"
)

func TestReindexExpiryIndexResetsState(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("unable to create test db: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("unable to create expiry index: %v", err)
	}

	err = db.Update(func(dbTx database.Tx) error {
		if err := idx.Create(dbTx); err != nil {
			return err
		}
		if err := dbPutTipHeightIndexed(dbTx, 123); err != nil {
			return err
		}
		if err := dbPutIndexVersion(dbTx, CurrentIndexVersion); err != nil {
			return err
		}

		op := wire.OutPoint{
			Hash:  chainhash.DoubleHashH([]byte("outpoint")),
			Index: 0,
		}
		if err := idx.connectTxOut(dbTx, &op, 10); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unable to seed expiry index state: %v", err)
	}

	if err := ReindexExpiryIndex(db); err != nil {
		t.Fatalf("reindex failed: %v", err)
	}

	err = db.View(func(dbTx database.Tx) error {
		tipHeight, err := dbGetTipHeightIndexed(dbTx)
		if err != nil {
			return err
		}
		if tipHeight != -1 {
			t.Fatalf("expected reset tip height -1, got %d", tipHeight)
		}

		version, err := dbGetIndexVersion(dbTx)
		if err != nil {
			return err
		}
		if version != CurrentIndexVersion {
			t.Fatalf("expected version %d, got %d", CurrentIndexVersion, version)
		}

		rows, hasMore, err := idx.ScanExpiringUTXOs(0, 1_000_000, 10, nil)
		if err != nil {
			return err
		}
		if hasMore {
			t.Fatal("expected cleared index to have no extra rows")
		}
		if len(rows) != 0 {
			t.Fatalf("expected cleared index to have no rows, got %d", len(rows))
		}

		snapshot, err := idx.GetAccumulatorSnapshot()
		if err != nil {
			return err
		}
		if snapshot.TipHeight != -1 {
			t.Fatalf("expected accumulator tip height -1, got %d", snapshot.TipHeight)
		}
		if snapshot.Root != NewMuHash().Digest() {
			t.Fatalf("expected identity accumulator root, got %x", snapshot.Root)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("unable to inspect expiry index state: %v", err)
	}
}
