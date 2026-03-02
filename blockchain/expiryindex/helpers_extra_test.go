package expiryindex

import (
	"strings"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btclog"
)

func TestChainAccessorNotSetErrors(t *testing.T) {
	db, teardown, err := createCoreTestDB()
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	defer teardown()

	idx, err := NewExpiryIndex(db, &chaincfg.ObtcRegTestParams)
	if err != nil {
		t.Fatalf("new index: %v", err)
	}

	// Without SetChainAccessor, these should return "chain accessor not set".
	if _, err := idx.getBlockByHeight(10); err == nil || !strings.Contains(err.Error(), "chain accessor not set") {
		t.Fatalf("expected chain accessor error from getBlockByHeight, got %v", err)
	}
	if _, err := idx.getSpentTxOuts(10); err == nil || !strings.Contains(err.Error(), "chain accessor not set") {
		t.Fatalf("expected chain accessor error from getSpentTxOuts, got %v", err)
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

	// Without chain accessor, falls back to curTipHeight.
	idx.curTipHeight = 321
	if got := idx.getChainTipHeight(); got != 321 {
		t.Fatalf("tip height mismatch: got %d", got)
	}

	UseLogger(btclog.Disabled)
	DisableLog()
}
