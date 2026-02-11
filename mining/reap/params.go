package reap

type REAPParams struct {
	Sort         SortMode
	MaxInputs    int
	WeightBudget int64
	ScanBatch    int
	BurnPolicy   BurnPolicy
	TaxNum       int64
	TaxDen       int64
}

func DefaultREAPParams(mode SortMode) REAPParams {
	return REAPParams{
		Sort:         mode,
		MaxInputs:    1000,
		WeightBudget: 400_000,
		ScanBatch:    10_000,
		BurnPolicy:   BurnPolicyOpReturn,
		TaxNum:       30,
		TaxDen:       100,
	}
}
