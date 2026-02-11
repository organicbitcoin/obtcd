package reap

const (
	estBaseWeight       int64 = 40
	estInputWeightUpper int64 = 600
	estBurnOutputWeight int64 = 172
	estMarkOutputWeight int64 = 120
)

func EstimateBlueprintWeight(numInputs int) int64 {
	if numInputs < 0 {
		numInputs = 0
	}
	return estBaseWeight + int64(numInputs)*estInputWeightUpper + estBurnOutputWeight + estMarkOutputWeight
}
