package indexers

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/gcs/builder"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	_ "github.com/btcsuite/btcd/database/ffldb"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

func createIndexersTestDB(t *testing.T, params *chaincfg.Params) database.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "ffldb")
	db, err := database.Create("ffldb", dbPath, params.Net)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	return db
}

func initIndexesAtTip(t *testing.T, db database.DB, tipHash *chainhash.Hash,
	tipHeight int32, indexes ...Indexer) {

	t.Helper()

	err := db.Update(func(dbTx database.Tx) error {
		meta := dbTx.Metadata()
		if _, err := meta.CreateBucketIfNotExists(indexTipsBucketName); err != nil {
			return err
		}

		for _, index := range indexes {
			if err := index.Create(dbTx); err != nil {
				return err
			}
			if err := dbPutIndexerTip(dbTx, index.Key(), tipHash, tipHeight); err != nil {
				return err
			}
		}

		return nil
	})
	require.NoError(t, err)

	for _, index := range indexes {
		require.NoError(t, index.Init())
	}
}

func testAddressAndScript(t *testing.T, params *chaincfg.Params,
	fill byte) (btcutil.Address, []byte) {

	t.Helper()

	hash := bytes.Repeat([]byte{fill}, 20)
	addr, err := btcutil.NewAddressPubKeyHash(hash, params)
	require.NoError(t, err)

	script, err := txscript.PayToAddrScript(addr)
	require.NoError(t, err)

	return addr, script
}

func makeCoinbaseTx(height int32, pkScript []byte, value int64) *wire.MsgTx {
	tx := wire.NewMsgTx(wire.TxVersion)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{
			Hash:  chainhash.Hash{},
			Index: wire.MaxPrevOutIndex,
		},
		SignatureScript: []byte{byte(height + 1)},
		Sequence:        wire.MaxTxInSequenceNum,
	})
	tx.AddTxOut(&wire.TxOut{Value: value, PkScript: pkScript})

	return tx
}

func makeSpendTx(prevHash chainhash.Hash, prevIndex uint32, outputScript []byte,
	outputValue int64) *wire.MsgTx {

	tx := wire.NewMsgTx(wire.TxVersion)
	tx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{Hash: prevHash, Index: prevIndex},
		SignatureScript:  []byte{txscript.OP_TRUE},
		Sequence:         wire.MaxTxInSequenceNum,
	})
	tx.AddTxOut(&wire.TxOut{Value: outputValue, PkScript: outputScript})

	return tx
}

func makeTestBlock(prevHash chainhash.Hash, nonce uint32, height int32,
	txs ...*wire.MsgTx) *btcutil.Block {

	msgBlock := &wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:   1,
			PrevBlock: prevHash,
			Timestamp: time.Unix(int64(height), 0).Add(time.Duration(nonce) * time.Second),
			Bits:      0x1d00ffff,
			Nonce:     nonce,
		},
	}
	for _, tx := range txs {
		msgBlock.AddTransaction(tx)
	}

	block := btcutil.NewBlock(msgBlock)
	block.SetHeight(height)
	return block
}

func storeBlock(t *testing.T, db database.DB, block *btcutil.Block) {
	t.Helper()

	err := db.Update(func(dbTx database.Tx) error {
		return dbTx.StoreBlock(block)
	})
	require.NoError(t, err)
}

func assertIndexerTip(t *testing.T, db database.DB, index Indexer,
	wantHash *chainhash.Hash, wantHeight int32) {

	t.Helper()

	err := db.View(func(dbTx database.Tx) error {
		gotHash, gotHeight, err := dbFetchIndexerTip(dbTx, index.Key())
		require.NoError(t, err)
		require.Equal(t, wantHeight, gotHeight)
		require.Equal(t, *wantHash, *gotHash)
		return nil
	})
	require.NoError(t, err)
}

func fetchTxHashFromRegion(t *testing.T, db database.DB,
	region *database.BlockRegion) *chainhash.Hash {

	t.Helper()

	var txHash chainhash.Hash
	err := db.View(func(dbTx database.Tx) error {
		txBytes, err := dbTx.FetchBlockRegion(region)
		if err != nil {
			return err
		}

		var msgTx wire.MsgTx
		if err := msgTx.Deserialize(bytes.NewReader(txBytes)); err != nil {
			return err
		}

		txHash = msgTx.TxHash()
		return nil
	})
	require.NoError(t, err)

	return &txHash
}

func addressIndexedTxHashes(t *testing.T, idx *AddrIndex, db database.DB,
	addr btcutil.Address) []chainhash.Hash {

	t.Helper()

	regions, skipped, err := idx.TxRegionsForAddress(nil, addr, 0, 10, false)
	require.NoError(t, err)
	require.Zero(t, skipped)

	hashes := make([]chainhash.Hash, 0, len(regions))
	for i := range regions {
		hashes = append(hashes, *fetchTxHashFromRegion(t, db, &regions[i]))
	}

	return hashes
}

func expectedFilterArtifacts(t *testing.T, block *btcutil.Block,
	prevScripts [][]byte, prevHeader chainhash.Hash) ([]byte, chainhash.Hash, chainhash.Hash) {

	t.Helper()

	filter, err := builder.BuildBasicFilter(block.MsgBlock(), prevScripts)
	require.NoError(t, err)

	filterBytes, err := filter.NBytes()
	require.NoError(t, err)

	filterHash, err := builder.GetFilterHash(filter)
	require.NoError(t, err)

	filterHeader, err := builder.MakeHeaderForFilter(filter, prevHeader)
	require.NoError(t, err)

	return filterBytes, filterHash, filterHeader
}

func TestManagerConnectDisconnectReorgMaintainsIndexes(t *testing.T) {
	params := &chaincfg.MainNetParams
	db := createIndexersTestDB(t, params)

	txIdx := NewTxIndex(db)
	addrIdx := NewAddrIndex(db, params)
	cfIdx := NewCfIndex(db, params)
	manager := NewManager(db, []Indexer{txIdx, addrIdx, cfIdx})

	var zeroHash chainhash.Hash
	initIndexesAtTip(t, db, &zeroHash, 0, txIdx, addrIdx, cfIdx)

	_, coinbase1Script := testAddressAndScript(t, params, 0x01)
	addrOut1, out1Script := testAddressAndScript(t, params, 0x02)
	_, prev1Script := testAddressAndScript(t, params, 0x03)

	_, coinbase2aScript := testAddressAndScript(t, params, 0x04)
	addrOut2a, out2aScript := testAddressAndScript(t, params, 0x05)
	_, prev2aScript := testAddressAndScript(t, params, 0x06)

	_, coinbase2bScript := testAddressAndScript(t, params, 0x07)
	addrOut2b, out2bScript := testAddressAndScript(t, params, 0x08)
	_, prev2bScript := testAddressAndScript(t, params, 0x09)

	coinbase1 := makeCoinbaseTx(1, coinbase1Script, 50_000)
	spend1 := makeSpendTx(chainhash.Hash{0xa1}, 0, out1Script, 40_000)
	block1 := makeTestBlock(zeroHash, 1, 1, coinbase1, spend1)
	stxos1 := []blockchain.SpentTxOut{{
		Amount:     50_000,
		PkScript:   prev1Script,
		Height:     0,
		IsCoinBase: false,
	}}

	coinbase2a := makeCoinbaseTx(2, coinbase2aScript, 50_000)
	spend2a := makeSpendTx(chainhash.Hash{0xa2}, 1, out2aScript, 30_000)
	block2a := makeTestBlock(*block1.Hash(), 2, 2, coinbase2a, spend2a)
	stxos2a := []blockchain.SpentTxOut{{
		Amount:     40_000,
		PkScript:   prev2aScript,
		Height:     1,
		IsCoinBase: false,
	}}

	coinbase2b := makeCoinbaseTx(2, coinbase2bScript, 50_000)
	spend2b := makeSpendTx(chainhash.Hash{0xa3}, 2, out2bScript, 31_000)
	block2b := makeTestBlock(*block1.Hash(), 3, 2, coinbase2b, spend2b)
	stxos2b := []blockchain.SpentTxOut{{
		Amount:     41_000,
		PkScript:   prev2bScript,
		Height:     1,
		IsCoinBase: false,
	}}

	storeBlock(t, db, block1)
	storeBlock(t, db, block2a)
	storeBlock(t, db, block2b)

	block1Filter, block1FilterHash, block1Header := expectedFilterArtifacts(
		t, block1, [][]byte{prev1Script}, zeroHash,
	)
	block2aFilter, block2aFilterHash, block2aHeader := expectedFilterArtifacts(
		t, block2a, [][]byte{prev2aScript}, block1Header,
	)
	block2bFilter, block2bFilterHash, block2bHeader := expectedFilterArtifacts(
		t, block2b, [][]byte{prev2bScript}, block1Header,
	)

	spend1Hash := spend1.TxHash()
	spend2aHash := spend2a.TxHash()
	spend2bHash := spend2b.TxHash()

	err := db.Update(func(dbTx database.Tx) error {
		return manager.ConnectBlock(dbTx, block1, stxos1)
	})
	require.NoError(t, err)
	err = db.Update(func(dbTx database.Tx) error {
		return manager.ConnectBlock(dbTx, block2a, stxos2a)
	})
	require.NoError(t, err)

	for _, index := range []Indexer{txIdx, addrIdx, cfIdx} {
		assertIndexerTip(t, db, index, block2a.Hash(), 2)
	}

	region1, err := txIdx.TxBlockRegion(&spend1Hash)
	require.NoError(t, err)
	require.NotNil(t, region1)
	require.Equal(t, *block1.Hash(), *region1.Hash)
	require.Equal(t, spend1Hash, *fetchTxHashFromRegion(t, db, region1))

	region2a, err := txIdx.TxBlockRegion(&spend2aHash)
	require.NoError(t, err)
	require.NotNil(t, region2a)
	require.Equal(t, *block2a.Hash(), *region2a.Hash)
	require.Equal(t, spend2aHash, *fetchTxHashFromRegion(t, db, region2a))

	require.Equal(t, []chainhash.Hash{spend1Hash},
		addressIndexedTxHashes(t, addrIdx, db, addrOut1))
	require.Equal(t, []chainhash.Hash{spend2aHash},
		addressIndexedTxHashes(t, addrIdx, db, addrOut2a))
	require.Empty(t, addressIndexedTxHashes(t, addrIdx, db, addrOut2b))

	gotFilter, err := cfIdx.FilterByBlockHash(block1.Hash(), wire.GCSFilterRegular)
	require.NoError(t, err)
	require.Equal(t, block1Filter, gotFilter)
	gotFilterHash, err := cfIdx.FilterHashByBlockHash(block1.Hash(), wire.GCSFilterRegular)
	require.NoError(t, err)
	require.Equal(t, block1FilterHash[:], gotFilterHash)
	gotFilterHeader, err := cfIdx.FilterHeaderByBlockHash(block1.Hash(), wire.GCSFilterRegular)
	require.NoError(t, err)
	require.Equal(t, block1Header[:], gotFilterHeader)

	gotFilter, err = cfIdx.FilterByBlockHash(block2a.Hash(), wire.GCSFilterRegular)
	require.NoError(t, err)
	require.Equal(t, block2aFilter, gotFilter)
	gotFilterHash, err = cfIdx.FilterHashByBlockHash(block2a.Hash(), wire.GCSFilterRegular)
	require.NoError(t, err)
	require.Equal(t, block2aFilterHash[:], gotFilterHash)
	gotFilterHeader, err = cfIdx.FilterHeaderByBlockHash(block2a.Hash(), wire.GCSFilterRegular)
	require.NoError(t, err)
	require.Equal(t, block2aHeader[:], gotFilterHeader)

	err = db.Update(func(dbTx database.Tx) error {
		return manager.DisconnectBlock(dbTx, block2a, stxos2a)
	})
	require.NoError(t, err)

	for _, index := range []Indexer{txIdx, addrIdx, cfIdx} {
		assertIndexerTip(t, db, index, block1.Hash(), 1)
	}

	region2a, err = txIdx.TxBlockRegion(&spend2aHash)
	require.NoError(t, err)
	require.Nil(t, region2a)
	region1, err = txIdx.TxBlockRegion(&spend1Hash)
	require.NoError(t, err)
	require.NotNil(t, region1)

	require.Equal(t, []chainhash.Hash{spend1Hash},
		addressIndexedTxHashes(t, addrIdx, db, addrOut1))
	require.Empty(t, addressIndexedTxHashes(t, addrIdx, db, addrOut2a))
	require.Empty(t, addressIndexedTxHashes(t, addrIdx, db, addrOut2b))

	gotFilter, err = cfIdx.FilterByBlockHash(block2a.Hash(), wire.GCSFilterRegular)
	require.NoError(t, err)
	require.Nil(t, gotFilter)
	gotFilterHash, err = cfIdx.FilterHashByBlockHash(block2a.Hash(), wire.GCSFilterRegular)
	require.NoError(t, err)
	require.Nil(t, gotFilterHash)
	gotFilterHeader, err = cfIdx.FilterHeaderByBlockHash(block2a.Hash(), wire.GCSFilterRegular)
	require.NoError(t, err)
	require.Nil(t, gotFilterHeader)

	err = db.Update(func(dbTx database.Tx) error {
		return manager.ConnectBlock(dbTx, block2b, stxos2b)
	})
	require.NoError(t, err)

	for _, index := range []Indexer{txIdx, addrIdx, cfIdx} {
		assertIndexerTip(t, db, index, block2b.Hash(), 2)
	}

	region2b, err := txIdx.TxBlockRegion(&spend2bHash)
	require.NoError(t, err)
	require.NotNil(t, region2b)
	require.Equal(t, *block2b.Hash(), *region2b.Hash)
	require.Equal(t, spend2bHash, *fetchTxHashFromRegion(t, db, region2b))

	region2a, err = txIdx.TxBlockRegion(&spend2aHash)
	require.NoError(t, err)
	require.Nil(t, region2a)

	require.Equal(t, []chainhash.Hash{spend1Hash},
		addressIndexedTxHashes(t, addrIdx, db, addrOut1))
	require.Empty(t, addressIndexedTxHashes(t, addrIdx, db, addrOut2a))
	require.Equal(t, []chainhash.Hash{spend2bHash},
		addressIndexedTxHashes(t, addrIdx, db, addrOut2b))

	gotFilter, err = cfIdx.FilterByBlockHash(block2b.Hash(), wire.GCSFilterRegular)
	require.NoError(t, err)
	require.Equal(t, block2bFilter, gotFilter)
	gotFilterHash, err = cfIdx.FilterHashByBlockHash(block2b.Hash(), wire.GCSFilterRegular)
	require.NoError(t, err)
	require.Equal(t, block2bFilterHash[:], gotFilterHash)
	gotFilterHeader, err = cfIdx.FilterHeaderByBlockHash(block2b.Hash(), wire.GCSFilterRegular)
	require.NoError(t, err)
	require.Equal(t, block2bHeader[:], gotFilterHeader)
}

func TestManagerConnectDisconnectRequiresTipAlignment(t *testing.T) {
	params := &chaincfg.MainNetParams
	db := createIndexersTestDB(t, params)

	txIdx := NewTxIndex(db)
	manager := NewManager(db, []Indexer{txIdx})

	var zeroHash chainhash.Hash
	initIndexesAtTip(t, db, &zeroHash, 0, txIdx)

	_, coinbaseScript := testAddressAndScript(t, params, 0x11)
	block := makeTestBlock(zeroHash, 1, 1, makeCoinbaseTx(1, coinbaseScript, 50_000))
	wrongPrevBlock := makeTestBlock(chainhash.Hash{0x22}, 2, 1,
		makeCoinbaseTx(1, coinbaseScript, 50_000))

	err := db.Update(func(dbTx database.Tx) error {
		return manager.ConnectBlock(dbTx, wrongPrevBlock, nil)
	})
	require.Error(t, err)
	require.IsType(t, AssertError(""), err)
	require.Contains(t, err.Error(), "extends the current index tip")

	err = db.Update(func(dbTx database.Tx) error {
		return manager.ConnectBlock(dbTx, block, nil)
	})
	require.NoError(t, err)

	err = db.Update(func(dbTx database.Tx) error {
		return manager.DisconnectBlock(dbTx, wrongPrevBlock, nil)
	})
	require.Error(t, err)
	require.IsType(t, AssertError(""), err)
	require.Contains(t, err.Error(), "current index tip")
}
