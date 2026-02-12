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

type REAPStats struct {
	Candidates int
	Picked     int
	Skipped    int
	EstWeight  int64
}

type REAPPlan struct {
	Inputs      []wire.OutPoint
	TaxTotal    int64
	RefundTotal int64
	Height      int32
	Stats       REAPStats
}

var (
	ErrNilView  = fmt.Errorf("nil utxo view")
	ErrNilIndex = fmt.Errorf("nil expiry index")
)
