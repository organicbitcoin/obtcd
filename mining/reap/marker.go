package reap

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"

	"github.com/btcsuite/btcd/wire"
)

// MarkerDigest returns hex-encoded sha256 digest of ordered REAP inputs.
//
// Serialization format per input:
//   - txid bytes (internal wire order)
//   - vout uint32 little-endian
func MarkerDigest(inputs []wire.OutPoint) string {
	h := sha256.New()
	var idx [4]byte
	for _, op := range inputs {
		h.Write(op.Hash[:])
		binary.LittleEndian.PutUint32(idx[:], op.Index)
		h.Write(idx[:])
	}
	return hex.EncodeToString(h.Sum(nil))
}
