package reap

import (
	"crypto/sha256"
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
	for _, op := range inputs {
		h.Write(op.Hash[:])
		var idx [4]byte
		idx[0] = byte(op.Index)
		idx[1] = byte(op.Index >> 8)
		idx[2] = byte(op.Index >> 16)
		idx[3] = byte(op.Index >> 24)
		h.Write(idx[:])
	}
	return hex.EncodeToString(h.Sum(nil))
}
