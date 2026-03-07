package expiryindex

import (
	"encoding/binary"
	"fmt"

	"github.com/btcsuite/btcd/database"
	"github.com/btcsuite/btcd/wire"
)

// AccumulatorDigestSize is the byte length of the MuHash digest (SHA256).
const AccumulatorDigestSize = 32

// computeEntryData builds the 44-byte canonical encoding of a UTXO entry
// for the MuHash accumulator: encodeOutPoint(36B) || encodeExpiryKey(8B).
func computeEntryData(op *wire.OutPoint, expiryKey uint64) []byte {
	data := make([]byte, 44)
	copy(data[0:32], op.Hash[:])
	binary.LittleEndian.PutUint32(data[32:36], op.Index)
	binary.BigEndian.PutUint64(data[36:44], expiryKey)
	return data
}

// dbGetAccumulatorState loads the MuHash accumulator from the metadata bucket.
// Returns identity MuHash if no state has been stored yet.
func dbGetAccumulatorState(dbTx database.Tx) (*MuHash, error) {
	data, err := dbGetIndexMeta(dbTx, keyAccumulatorState)
	if err != nil {
		return nil, fmt.Errorf("failed to read accumulator state: %v", err)
	}

	mh := NewMuHash()
	if data == nil {
		return mh, nil // identity state
	}

	if err := mh.Deserialize(data); err != nil {
		return nil, fmt.Errorf("failed to deserialize accumulator: %v", err)
	}
	return mh, nil
}

// dbPutAccumulatorState stores the MuHash accumulator in the metadata bucket.
func dbPutAccumulatorState(dbTx database.Tx, mh *MuHash) error {
	return dbPutIndexMeta(dbTx, keyAccumulatorState, mh.Serialize())
}
