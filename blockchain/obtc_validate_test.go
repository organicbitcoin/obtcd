package blockchain

import (
	"encoding/hex"
	"testing"
	"time"

	mock_blockchain "github.com/organicbitcoin/obtcd/blockchain/mock"
	"github.com/organicbitcoin/obtcd/chaincfg"
	"github.com/organicbitcoin/obtcd/chaincfg/chainhash"
	"github.com/organicbitcoin/obtcd/utxo"
	"github.com/organicbitcoin/obtcd/wire"
	"github.com/organicbitcoin/btcutil"
	"github.com/golang/mock/gomock"
)

// BlockWithTaxTxs defines a block that contain tax transactions.  It is used to test tax txs operations.
// This block is
var BlockWithTaxTxs = wire.MsgBlock{
	Header: wire.BlockHeader{
		Version: 1,
		PrevBlock: chainhash.Hash([32]byte{ // Make go vet happy.
			0x50, 0x12, 0x01, 0x19, 0x17, 0x2a, 0x61, 0x04,
			0x21, 0xa6, 0xc3, 0x01, 0x1d, 0xd3, 0x30, 0xd9,
			0xdf, 0x07, 0xb6, 0x36, 0x16, 0xc2, 0xcc, 0x1f,
			0x1c, 0xd0, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00,
		}), // 000000000002d01c1fccc21636b607dfd930d31d01c3a62104612a1719011250
		MerkleRoot: chainhash.Hash([32]byte{ // Make go vet happy.
			0x66, 0x57, 0xa9, 0x25, 0x2a, 0xac, 0xd5, 0xc0,
			0xb2, 0x94, 0x09, 0x96, 0xec, 0xff, 0x95, 0x22,
			0x28, 0xc3, 0x06, 0x7c, 0xc3, 0x8d, 0x48, 0x85,
			0xef, 0xb5, 0xa4, 0xac, 0x42, 0x47, 0xe9, 0xf3,
		}), // f3e94742aca4b5ef85488dc37c06c3282295ffec960994b2c0d5ac2a25a95766
		Timestamp: time.Unix(1293623863, 0), // 2010-12-29 11:57:43 +0000 UTC
		Bits:      0x1b04864c,               // 453281356
		Nonce:     0x10572b0f,               // 274148111
	},
	Transactions: []*wire.MsgTx{
		{
			Version: 1,
			Type:    0x11,
			TxIn: []*wire.TxIn{
				{
					PreviousOutPoint: wire.OutPoint{
						Hash:  chainhash.Hash{},
						Index: 0xffffffff,
					},
					SignatureScript: []byte{
						0x04, 0x4c, 0x86, 0x04, 0x1b, 0x02, 0x06, 0x02,
					},
					Sequence: 0xffffffff,
				},
			},
			TxOut: []*wire.TxOut{
				{
					Value: 0x12a05f200, // 5000000000
					PkScript: []byte{
						0x41, // OP_DATA_65
						0x04, 0x1b, 0x0e, 0x8c, 0x25, 0x67, 0xc1, 0x25,
						0x36, 0xaa, 0x13, 0x35, 0x7b, 0x79, 0xa0, 0x73,
						0xdc, 0x44, 0x44, 0xac, 0xb8, 0x3c, 0x4e, 0xc7,
						0xa0, 0xe2, 0xf9, 0x9d, 0xd7, 0x45, 0x75, 0x16,
						0xc5, 0x81, 0x72, 0x42, 0xda, 0x79, 0x69, 0x24,
						0xca, 0x4e, 0x99, 0x94, 0x7d, 0x08, 0x7f, 0xed,
						0xf9, 0xce, 0x46, 0x7c, 0xb9, 0xf7, 0xc6, 0x28,
						0x70, 0x78, 0xf8, 0x01, 0xdf, 0x27, 0x6f, 0xdf,
						0x84, // 65-byte signature
						0xac, // OP_CHECKSIG
					},
				},
			},
			LockTime: 0,
		},
		{
			Version: 1,
			Type:    0x01,
			TxIn: []*wire.TxIn{
				{
					PreviousOutPoint: wire.OutPoint{
						Hash: chainhash.Hash([32]byte{ // Make go vet happy.
							0x03, 0x2e, 0x38, 0xe9, 0xc0, 0xa8, 0x4c, 0x60,
							0x46, 0xd6, 0x87, 0xd1, 0x05, 0x56, 0xdc, 0xac,
							0xc4, 0x1d, 0x27, 0x5e, 0xc5, 0x5f, 0xc0, 0x07,
							0x79, 0xac, 0x88, 0xfd, 0xf3, 0x57, 0xa1, 0x87,
						}), // 87a157f3fd88ac7907c05fc55e271dc4acdc5605d187d646604ca8c0e9382e03
						Index: 0,
					},
					SignatureScript: []byte{
						0x49, // OP_DATA_73
						0x30, 0x46, 0x02, 0x21, 0x00, 0xc3, 0x52, 0xd3,
						0xdd, 0x99, 0x3a, 0x98, 0x1b, 0xeb, 0xa4, 0xa6,
						0x3a, 0xd1, 0x5c, 0x20, 0x92, 0x75, 0xca, 0x94,
						0x70, 0xab, 0xfc, 0xd5, 0x7d, 0xa9, 0x3b, 0x58,
						0xe4, 0xeb, 0x5d, 0xce, 0x82, 0x02, 0x21, 0x00,
						0x84, 0x07, 0x92, 0xbc, 0x1f, 0x45, 0x60, 0x62,
						0x81, 0x9f, 0x15, 0xd3, 0x3e, 0xe7, 0x05, 0x5c,
						0xf7, 0xb5, 0xee, 0x1a, 0xf1, 0xeb, 0xcc, 0x60,
						0x28, 0xd9, 0xcd, 0xb1, 0xc3, 0xaf, 0x77, 0x48,
						0x01, // 73-byte signature
						0x41, // OP_DATA_65
						0x04, 0xf4, 0x6d, 0xb5, 0xe9, 0xd6, 0x1a, 0x9d,
						0xc2, 0x7b, 0x8d, 0x64, 0xad, 0x23, 0xe7, 0x38,
						0x3a, 0x4e, 0x6c, 0xa1, 0x64, 0x59, 0x3c, 0x25,
						0x27, 0xc0, 0x38, 0xc0, 0x85, 0x7e, 0xb6, 0x7e,
						0xe8, 0xe8, 0x25, 0xdc, 0xa6, 0x50, 0x46, 0xb8,
						0x2c, 0x93, 0x31, 0x58, 0x6c, 0x82, 0xe0, 0xfd,
						0x1f, 0x63, 0x3f, 0x25, 0xf8, 0x7c, 0x16, 0x1b,
						0xc6, 0xf8, 0xa6, 0x30, 0x12, 0x1d, 0xf2, 0xb3,
						0xd3, // 65-byte pubkey
					},
					Sequence: 0xffffffff,
				},
			},
			TxOut: []*wire.TxOut{
				{
					Value: 0x2123e300, // 556000000
					PkScript: []byte{
						0x76, // OP_DUP
						0xa9, // OP_HASH160
						0x14, // OP_DATA_20
						0xc3, 0x98, 0xef, 0xa9, 0xc3, 0x92, 0xba, 0x60,
						0x13, 0xc5, 0xe0, 0x4e, 0xe7, 0x29, 0x75, 0x5e,
						0xf7, 0xf5, 0x8b, 0x32,
						0x88, // OP_EQUALVERIFY
						0xac, // OP_CHECKSIG
					},
				},
				{
					Value: 0x108e20f00, // 4444000000
					PkScript: []byte{
						0x76, // OP_DUP
						0xa9, // OP_HASH160
						0x14, // OP_DATA_20
						0x94, 0x8c, 0x76, 0x5a, 0x69, 0x14, 0xd4, 0x3f,
						0x2a, 0x7a, 0xc1, 0x77, 0xda, 0x2c, 0x2f, 0x6b,
						0x52, 0xde, 0x3d, 0x7c,
						0x88, // OP_EQUALVERIFY
						0xac, // OP_CHECKSIG
					},
				},
			},
			LockTime: 0,
		},
		{
			Version: 1,
			Type:    0x01,
			TxIn: []*wire.TxIn{
				{
					PreviousOutPoint: wire.OutPoint{
						Hash: chainhash.Hash([32]byte{ // Make go vet happy.
							0xc3, 0x3e, 0xbf, 0xf2, 0xa7, 0x09, 0xf1, 0x3d,
							0x9f, 0x9a, 0x75, 0x69, 0xab, 0x16, 0xa3, 0x27,
							0x86, 0xaf, 0x7d, 0x7e, 0x2d, 0xe0, 0x92, 0x65,
							0xe4, 0x1c, 0x61, 0xd0, 0x78, 0x29, 0x4e, 0xcf,
						}), // cf4e2978d0611ce46592e02d7e7daf8627a316ab69759a9f3df109a7f2bf3ec3
						Index: 1,
					},
					SignatureScript: []byte{
						0x47, // OP_DATA_71
						0x30, 0x44, 0x02, 0x20, 0x03, 0x2d, 0x30, 0xdf,
						0x5e, 0xe6, 0xf5, 0x7f, 0xa4, 0x6c, 0xdd, 0xb5,
						0xeb, 0x8d, 0x0d, 0x9f, 0xe8, 0xde, 0x6b, 0x34,
						0x2d, 0x27, 0x94, 0x2a, 0xe9, 0x0a, 0x32, 0x31,
						0xe0, 0xba, 0x33, 0x3e, 0x02, 0x20, 0x3d, 0xee,
						0xe8, 0x06, 0x0f, 0xdc, 0x70, 0x23, 0x0a, 0x7f,
						0x5b, 0x4a, 0xd7, 0xd7, 0xbc, 0x3e, 0x62, 0x8c,
						0xbe, 0x21, 0x9a, 0x88, 0x6b, 0x84, 0x26, 0x9e,
						0xae, 0xb8, 0x1e, 0x26, 0xb4, 0xfe, 0x01,
						0x41, // OP_DATA_65
						0x04, 0xae, 0x31, 0xc3, 0x1b, 0xf9, 0x12, 0x78,
						0xd9, 0x9b, 0x83, 0x77, 0xa3, 0x5b, 0xbc, 0xe5,
						0xb2, 0x7d, 0x9f, 0xff, 0x15, 0x45, 0x68, 0x39,
						0xe9, 0x19, 0x45, 0x3f, 0xc7, 0xb3, 0xf7, 0x21,
						0xf0, 0xba, 0x40, 0x3f, 0xf9, 0x6c, 0x9d, 0xee,
						0xb6, 0x80, 0xe5, 0xfd, 0x34, 0x1c, 0x0f, 0xc3,
						0xa7, 0xb9, 0x0d, 0xa4, 0x63, 0x1e, 0xe3, 0x95,
						0x60, 0x63, 0x9d, 0xb4, 0x62, 0xe9, 0xcb, 0x85,
						0x0f, // 65-byte pubkey
					},
					Sequence: 0xffffffff,
				},
			},
			TxOut: []*wire.TxOut{
				{
					Value: 0xf4240, // 1000000
					PkScript: []byte{
						0x76, // OP_DUP
						0xa9, // OP_HASH160
						0x14, // OP_DATA_20
						0xb0, 0xdc, 0xbf, 0x97, 0xea, 0xbf, 0x44, 0x04,
						0xe3, 0x1d, 0x95, 0x24, 0x77, 0xce, 0x82, 0x2d,
						0xad, 0xbe, 0x7e, 0x10,
						0x88, // OP_EQUALVERIFY
						0xac, // OP_CHECKSIG
					},
				},
				{
					Value: 0x11d260c0, // 299000000
					PkScript: []byte{
						0x76, // OP_DUP
						0xa9, // OP_HASH160
						0x14, // OP_DATA_20
						0x6b, 0x12, 0x81, 0xee, 0xc2, 0x5a, 0xb4, 0xe1,
						0xe0, 0x79, 0x3f, 0xf4, 0xe0, 0x8a, 0xb1, 0xab,
						0xb3, 0x40, 0x9c, 0xd9,
						0x88, // OP_EQUALVERIFY
						0xac, // OP_CHECKSIG
					},
				},
			},
			LockTime: 0,
		},
		{
			Version: 1,
			Type:    0x11,
			TxIn: []*wire.TxIn{
				{
					PreviousOutPoint: wire.OutPoint{
						Hash: chainhash.Hash([32]byte{ // Make go vet happy.
							0x0b, 0x60, 0x72, 0xb3, 0x86, 0xd4, 0xa7, 0x73,
							0x23, 0x52, 0x37, 0xf6, 0x4c, 0x11, 0x26, 0xac,
							0x3b, 0x24, 0x0c, 0x84, 0xb9, 0x17, 0xa3, 0x90,
							0x9b, 0xa1, 0xc4, 0x3d, 0xed, 0x5f, 0x51, 0xf4,
						}), // f4515fed3dc4a19b90a317b9840c243bac26114cf637522373a7d486b372600b
						Index: 0,
					},
					SignatureScript: []byte{
						0x49, // OP_DATA_73
						0x30, 0x46, 0x02, 0x21, 0x00, 0xbb, 0x1a, 0xd2,
						0x6d, 0xf9, 0x30, 0xa5, 0x1c, 0xce, 0x11, 0x0c,
						0xf4, 0x4f, 0x7a, 0x48, 0xc3, 0xc5, 0x61, 0xfd,
						0x97, 0x75, 0x00, 0xb1, 0xae, 0x5d, 0x6b, 0x6f,
						0xd1, 0x3d, 0x0b, 0x3f, 0x4a, 0x02, 0x21, 0x00,
						0xc5, 0xb4, 0x29, 0x51, 0xac, 0xed, 0xff, 0x14,
						0xab, 0xba, 0x27, 0x36, 0xfd, 0x57, 0x4b, 0xdb,
						0x46, 0x5f, 0x3e, 0x6f, 0x8d, 0xa1, 0x2e, 0x2c,
						0x53, 0x03, 0x95, 0x4a, 0xca, 0x7f, 0x78, 0xf3,
						0x01, // 73-byte signature
						0x41, // OP_DATA_65
						0x04, 0xa7, 0x13, 0x5b, 0xfe, 0x82, 0x4c, 0x97,
						0xec, 0xc0, 0x1e, 0xc7, 0xd7, 0xe3, 0x36, 0x18,
						0x5c, 0x81, 0xe2, 0xaa, 0x2c, 0x41, 0xab, 0x17,
						0x54, 0x07, 0xc0, 0x94, 0x84, 0xce, 0x96, 0x94,
						0xb4, 0x49, 0x53, 0xfc, 0xb7, 0x51, 0x20, 0x65,
						0x64, 0xa9, 0xc2, 0x4d, 0xd0, 0x94, 0xd4, 0x2f,
						0xdb, 0xfd, 0xd5, 0xaa, 0xd3, 0xe0, 0x63, 0xce,
						0x6a, 0xf4, 0xcf, 0xaa, 0xea, 0x4e, 0xa1, 0x4f,
						0xbb, // 65-byte pubkey
					},
					Sequence: 0xffffffff,
				},
			},
			TxOut: []*wire.TxOut{
				{
					Value: 0xf4240, // 1000000
					PkScript: []byte{
						0x76, // OP_DUP
						0xa9, // OP_HASH160
						0x14, // OP_DATA_20
						0x39, 0xaa, 0x3d, 0x56, 0x9e, 0x06, 0xa1, 0xd7,
						0x92, 0x6d, 0xc4, 0xbe, 0x11, 0x93, 0xc9, 0x9b,
						0xf2, 0xeb, 0x9e, 0xe0,
						0x88, // OP_EQUALVERIFY
						0xac, // OP_CHECKSIG
					},
				},
			},
			LockTime: 0,
		},
	},
}

func TestFetchTaxTransactions(t *testing.T) {
	blockWithTaxTxs := btcutil.NewBlock(&BlockWithTaxTxs)
	taxTxsArray1 := FetchTaxTransactions(blockWithTaxTxs)
	// taxTxsArray1 should contain two elements
	if len(taxTxsArray1) != 2 {
		t.Fatalf("taxTxsArray1 should contain two elements: %v\n", taxTxsArray1)
	}

	block100000 := btcutil.NewBlock(&Block100000)
	taxTxsArray2 := FetchTaxTransactions(block100000)
	// taxTxsArray1 should be empty
	if len(taxTxsArray2) != 0 {
		t.Fatalf("taxTxsArray2 should return empty result: %v\n", taxTxsArray2)
	}
}

func TestFetchPrevBlockHasTaxTxs(t *testing.T) {
	block100000 := btcutil.NewBlock(&Block100000)
	block100000.SetHeight(int32(100000))
	blockWithTaxTxs := btcutil.NewBlock(&BlockWithTaxTxs)
	blockWithTaxTxs.SetHeight(int32(99999))

	ctl := gomock.NewController(t)
	mockedBlockchain := mock_blockchain.NewMockInterface(ctl)
	mockedBlockchain.EXPECT().BlockByHeight(int32(99999)).Return(blockWithTaxTxs, nil)
	defer ctl.Finish()

	resultBlock, err := FetchPrevBlockHasTaxTxs(mockedBlockchain, block100000)
	if err != nil {
		t.Fatalf("Error in FetchPrevBlockHasTaxTxs")
	}
	if resultBlock != blockWithTaxTxs {
		t.Fatalf("FetchPrevBlockHasTaxTxs returns wrong result")
	}
}

func TestFetchHighestTaxTxInputHeight(t *testing.T) {
	chain, teardownFunc, err := chainSetup("FetchHighestTaxTxInputHeight", &chaincfg.MainNetParams)
	if err != nil {
		t.Errorf("Failed to setup chain instance: %v", err)
		return
	}
	defer teardownFunc()

	ctl := gomock.NewController(t)
	mockedUtxoViewPoint := mock_blockchain.NewMockUtxoViewpointInterface(ctl)
	mockedUtxoViewPoint.EXPECT().LookupEntry(
		wire.OutPoint{Hash: chainhash.Hash{},
			Index: 0xffffffff,
		},
	).Return(&utxo.UtxoEntry{BlockHeight: int32(90)})
	mockedUtxoViewPoint.EXPECT().LookupEntry(
		wire.OutPoint{Hash: chainhash.Hash([32]byte{
			0x0b, 0x60, 0x72, 0xb3, 0x86, 0xd4, 0xa7, 0x73,
			0x23, 0x52, 0x37, 0xf6, 0x4c, 0x11, 0x26, 0xac,
			0x3b, 0x24, 0x0c, 0x84, 0xb9, 0x17, 0xa3, 0x90,
			0x9b, 0xa1, 0xc4, 0x3d, 0xed, 0x5f, 0x51, 0xf4,
		}),
			Index: 0,
		},
	).Return(&utxo.UtxoEntry{BlockHeight: int32(100)})

	blockWithTaxTxs := btcutil.NewBlock(&BlockWithTaxTxs)

	chain.fetchHighestTaxTxInputHeight(blockWithTaxTxs, mockedUtxoViewPoint)
}

func TestCheckTxTaxAmount(t *testing.T) {
	ctl := gomock.NewController(t)
	mockedUtxoViewPoint := mock_blockchain.NewMockUtxoViewpointInterface(ctl)

	// PK1 contains addr: 12hnbu1xVuYF4VhaXYFyw6Pq2eudj3CG4g
	pk1, _ := hex.DecodeString("76a91412aed4a2fed565f0f473b3688e60246576710f2a88ac")
	// Normal amount: 10 times dust
	inputAmount1 := int64(chaincfg.MainNetParams.DustSatoshiAmount) * int64(10)
	// PK2 contains addr: 1ECRPUBJECFWcB73R6ydUQNJ6Ane8qYPr2
	pk2, _ := hex.DecodeString("76a91490c2917e7a89f3ec8a1bb82db92661dcab14fcc488ac")
	// Dust utxo
	inputAmount2 := int64(chaincfg.MainNetParams.DustSatoshiAmount)
	// PK3 contains multiple address
	// addr1: 020fa7bed1b89df218a2ed2c94ebbf872a7bda0f48d231eb8cb6f16b87d9bb5211
	// addr2: 02d7e287092457f2bea226cd7537c5ee99af50cca923795a2ea65cf249f783c5d1
	// addr3: 02e8b48f3c0a7c452792fa96cdcf2fc6a23298f4d6512bd8aa9a25210b66a1d450
	pk3, _ := hex.DecodeString("5221020fa7bed1b89df218a2ed2c94ebbf872a7bda0f48d231eb8cb6f16b87d9bb52112102d7e287092457f2bea226cd7537c5ee99af50cca923795a2ea65cf249f783c5d12102e8b48f3c0a7c452792fa96cdcf2fc6a23298f4d6512bd8aa9a25210b66a1d45053ae")
	inputAmount3 := int64(chaincfg.MainNetParams.DustSatoshiAmount) * int64(30)

	utxo1 := &utxo.UtxoEntry{
		Amount:      inputAmount1,
		BlockHeight: 6000,
		PkScript:    pk1,
	}
	utxo2 := &utxo.UtxoEntry{
		Amount:      inputAmount2,
		BlockHeight: 6000,
		PkScript:    pk2,
	}
	utxo3 := &utxo.UtxoEntry{
		Amount:      inputAmount3,
		BlockHeight: 6000,
		PkScript:    pk3,
	}

	// Tax transaction
	taxTx := wire.NewMsgTx(int32(1))
	taxTx.AddTxIn(
		&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash([32]byte{0x01}),
				Index: 1,
			},
		},
	)
	taxTx.AddTxIn(
		&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash([32]byte{0x02}),
				Index: 2,
			},
		},
	)
	taxTx.AddTxIn(
		&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash([32]byte{0x03}),
				Index: 3,
			},
		},
	)
	taxTx.AddTxOut(
		&wire.TxOut{
			Value:    inputAmount1 - int64(float64(inputAmount1*int64(chaincfg.MainNetParams.TaxRate)/100)),
			PkScript: pk1,
		},
	)
	taxTx.AddTxOut(
		&wire.TxOut{
			Value:    inputAmount3 - int64(float64(inputAmount3*int64(chaincfg.MainNetParams.TaxRate)/100)),
			PkScript: pk3,
		},
	)
	tx := btcutil.NewTx(taxTx)

	mockedUtxoViewPoint.EXPECT().LookupEntry(
		wire.OutPoint{
			Hash:  chainhash.Hash([32]byte{0x01}),
			Index: 1,
		},
	).Return(utxo1)

	mockedUtxoViewPoint.EXPECT().LookupEntry(
		wire.OutPoint{
			Hash:  chainhash.Hash([32]byte{0x02}),
			Index: 2,
		},
	).Return(utxo2)

	mockedUtxoViewPoint.EXPECT().LookupEntry(
		wire.OutPoint{
			Hash:  chainhash.Hash([32]byte{0x03}),
			Index: 3,
		},
	).Return(utxo3)

	totalTaxAmount, err := checkTxTaxAmount(tx, mockedUtxoViewPoint, &chaincfg.MainNetParams)
	if err != nil {
		t.Errorf("something went wrong: %v", err)
	} else {
		t.Logf("totalTaxAmount: %d", totalTaxAmount)
	}
}

func TestFetchAndValidateExpiredUtxosAndLargestHeight(t *testing.T) {
	ctl := gomock.NewController(t)
	mockedUtxoViewPoint := mock_blockchain.NewMockUtxoViewpointInterface(ctl)
	utxo11 := &utxo.UtxoEntry{
		Amount:      12345678,
		BlockHeight: 5000,
		PackedFlags: utxo.TfExpired,
	}
	utxo12 := &utxo.UtxoEntry{
		Amount:      12345678,
		BlockHeight: 6000,
		PackedFlags: utxo.TfExpired,
	}
	utxo21 := &utxo.UtxoEntry{
		Amount:      12345678,
		BlockHeight: 7000,
		PackedFlags: utxo.TfExpired,
	}
	utxo22 := &utxo.UtxoEntry{
		Amount:      12345678,
		BlockHeight: 8000,
		PackedFlags: utxo.TfExpired,
	}

	// Tax transaction
	taxTx1 := wire.NewMsgTx(int32(1))
	taxTx1.AddTxIn(
		&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash([32]byte{0x11}),
				Index: 1,
			},
		},
	)
	taxTx1.AddTxIn(
		&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash([32]byte{0x12}),
				Index: 2,
			},
		},
	)

	taxTx2 := wire.NewMsgTx(int32(1))
	taxTx2.AddTxIn(
		&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash([32]byte{0x21}),
				Index: 1,
			},
		},
	)
	taxTx2.AddTxIn(
		&wire.TxIn{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash([32]byte{0x22}),
				Index: 2,
			},
		},
	)
	tx1 := btcutil.NewTx(taxTx1)
	tx2 := btcutil.NewTx(taxTx2)

	// Build taxTxs input
	taxTxs := make([]*btcutil.Tx, 2)
	taxTxs[0] = tx1
	taxTxs[1] = tx2

	mockedUtxoViewPoint.EXPECT().LookupEntry(
		wire.OutPoint{
			Hash:  chainhash.Hash([32]byte{0x11}),
			Index: 1,
		},
	).Return(utxo11)
	mockedUtxoViewPoint.EXPECT().LookupEntry(
		wire.OutPoint{
			Hash:  chainhash.Hash([32]byte{0x12}),
			Index: 2,
		},
	).Return(utxo12)
	mockedUtxoViewPoint.EXPECT().LookupEntry(
		wire.OutPoint{
			Hash:  chainhash.Hash([32]byte{0x21}),
			Index: 1,
		},
	).Return(utxo21)
	mockedUtxoViewPoint.EXPECT().LookupEntry(
		wire.OutPoint{
			Hash:  chainhash.Hash([32]byte{0x22}),
			Index: 2,
		},
	).Return(utxo22)

	expectedUtxos, expectedHeight, error := fetchAndValidateExpiredUtxosAndLargestHeight(taxTxs, mockedUtxoViewPoint)

	if error != nil {
		t.Errorf("something went wrong when testing FetchAndValidateExpiredUtxosAndLargestHeight %v", error)
	}

	if len(expectedUtxos) != 4 {
		t.Errorf("Expired UTXOs are not correct: %v", expectedUtxos)
	}

	if expectedHeight != 8000 {
		t.Errorf("Expired largest height is incorrect: %v", expectedHeight)
	}
}

func TestFetchUtxosInRange(t *testing.T) {
	utxo1 := &utxo.UtxoEntry{
		Amount:      12345678,
		BlockHeight: 5000,
	}
	op1 := wire.OutPoint{
		Hash:  chainhash.Hash([32]byte{0x01}),
		Index: 1,
	}
	utxos1 := make(map[wire.OutPoint]*utxo.UtxoEntry)
	utxos1[op1] = utxo1

	utxo2 := &utxo.UtxoEntry{
		Amount:      12345678,
		BlockHeight: 5001,
	}
	op2 := wire.OutPoint{
		Hash:  chainhash.Hash([32]byte{0x02}),
		Index: 1,
	}
	utxos2 := make(map[wire.OutPoint]*utxo.UtxoEntry)
	utxos2[op2] = utxo2

	ctl := gomock.NewController(t)
	mockedBlockchain := mock_blockchain.NewMockInterface(ctl)
	mockedBlockchain.EXPECT().FetchUtxosByHeight(int32(5000)).Return(utxos1, nil)
	mockedBlockchain.EXPECT().FetchUtxosByHeight(int32(5001)).Return(utxos2, nil)
	defer ctl.Finish()

	resultUtxos, err := FetchUtxosInRange(mockedBlockchain, 5000, 5001)

	if err != nil {
		t.Errorf("something went wrong when testing FetchUtxosInRange %v", err)
	}

	if len(resultUtxos) != 2 {
		t.Errorf("FetchUtxosInRange should return all utxos by the given range, but only return %v", len(resultUtxos))
	}
}
