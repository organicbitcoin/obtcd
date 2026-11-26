// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package indexers

import (
	"errors"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
)

type testTipSourceIndexer struct {
	hash   *chainhash.Hash
	height int32
	err    error
}

func (*testTipSourceIndexer) Key() []byte  { return []byte("test-tip-source") }
func (*testTipSourceIndexer) Name() string { return "test tip source" }
func (*testTipSourceIndexer) Create(database.Tx) error {
	return nil
}
func (*testTipSourceIndexer) Init() error { return nil }
func (*testTipSourceIndexer) ConnectBlock(database.Tx, *btcutil.Block,
	[]blockchain.SpentTxOut) error {

	return nil
}
func (*testTipSourceIndexer) DisconnectBlock(database.Tx, *btcutil.Block,
	[]blockchain.SpentTxOut) error {

	return nil
}
func (idx *testTipSourceIndexer) IndexTip() (*chainhash.Hash, int32, error) {
	return idx.hash, idx.height, idx.err
}

func TestManagerSyncIndexerTip(t *testing.T) {
	db := createIndexersTestDB(t, &chaincfg.MainNetParams)
	wantHash := chainhash.Hash{0x72, 0x48}
	idx := &testTipSourceIndexer{hash: &wantHash, height: 956542}
	manager := NewManager(db, []Indexer{idx})

	err := db.Update(func(dbTx database.Tx) error {
		meta := dbTx.Metadata()
		if _, err := meta.CreateBucketIfNotExists(indexTipsBucketName); err != nil {
			return err
		}
		return manager.maybeCreateIndexes(dbTx)
	})
	if err != nil {
		t.Fatalf("create index: %v", err)
	}

	if err := manager.syncIndexerTip(idx); err != nil {
		t.Fatalf("sync index tip: %v", err)
	}
	assertIndexerTip(t, db, idx, &wantHash, 956542)
}

func TestManagerSyncIndexerTipRejectsInvalidSource(t *testing.T) {
	db := createIndexersTestDB(t, &chaincfg.MainNetParams)
	manager := NewManager(db, nil)

	tests := []struct {
		name string
		idx  *testTipSourceIndexer
		want string
	}{
		{
			name: "source error",
			idx:  &testTipSourceIndexer{err: errors.New("tip unavailable")},
			want: "tip unavailable",
		},
		{
			name: "nil hash",
			idx:  &testTipSourceIndexer{height: 1},
			want: "nil hash",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := manager.syncIndexerTip(test.idx)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got error %v, want containing %q", err, test.want)
			}
		})
	}
}
