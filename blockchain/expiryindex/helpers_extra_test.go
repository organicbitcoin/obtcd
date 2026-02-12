package expiryindex

import (
	"strings"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/database"
	"github.com/btcsuite/btclog"
)

func TestPlaceholderHelpersReturnExpectedErrors(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("new index: %v", err)
	}

	if _, err := idx.getBlockByHeight(10); err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("expected not implemented error from getBlockByHeight, got %v", err)
	}
	if _, err := idx.getSpentTxOuts(10); err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("expected not implemented error from getSpentTxOuts, got %v", err)
	}
	if _, _, err := idx.parseUTXOEntry([]byte{1}, []byte{2}); err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("expected not implemented error from parseUTXOEntry, got %v", err)
	}

	err = db.View(func(dbTx database.Tx) error {
		_, err := idx.getUTXOBucket(dbTx)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("expected not implemented error from getUTXOBucket, got %v", err)
	}
}

func TestGetChainTipHeightAndUseLogger(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("new index: %v", err)
	}
	idx.curTipHeight = 321
	if got := idx.getChainTipHeight(); got != 321 {
		t.Fatalf("tip height mismatch: got %d", got)
	}

	UseLogger(btclog.Disabled)
	DisableLog()
}
