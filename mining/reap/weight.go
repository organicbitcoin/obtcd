package reap

const (
	estBaseWeight         int64 = 40
	estInputWeightUpper   int64 = 600
	estRefundOutputWeight int64 = 172
	estMarkOutputWeight   int64 = 120
)

// EstimateBlueprintWeight returns a conservative upper bound for a blueprint
// with numInputs inputs. Refund outputs are conservatively modeled as one per
// input plus one marker output.
func EstimateBlueprintWeight(numInputs int) int64 {
	if numInputs < 0 {
		numInputs = 0
	}
	return estBaseWeight + int64(numInputs)*estInputWeightUpper + int64(numInputs)*estRefundOutputWeight + estMarkOutputWeight
}
