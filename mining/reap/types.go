package reap

import (
	"fmt"

	"github.com/btcsuite/btcd/wire"
)

type SortMode int

const (
	SortModeStrict SortMode = iota
	SortModeSimple
)

type BurnPolicy int

const (
	BurnPolicyOpReturn BurnPolicy = iota
	BurnPolicyP2WSHZero
	BurnPolicyP2TRNullKey
)

type REAPStats struct {
	Candidates int
	Picked     int
	Skipped    int
	EstWeight  int64
}

type REAPPlan struct {
	Inputs    []wire.OutPoint
	TaxTotal  int64
	BurnTotal int64
	Height    int32
	Stats     REAPStats
}

var (
	ErrNilView  = fmt.Errorf("nil utxo view")
	ErrNilIndex = fmt.Errorf("nil expiry index")
)
