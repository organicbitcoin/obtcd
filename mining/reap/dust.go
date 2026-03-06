package reap

// applyDustRule folds sub-threshold input values back into tax.
// If dustThresholdSat <= 0, the rule is disabled.
func applyDustRule(value, refund, tax, dustThresholdSat int64) (adjRefund, adjTax int64) {
	if dustThresholdSat <= 0 {
		return refund, tax
	}
	if value > 0 && value < dustThresholdSat {
		return 0, value
	}
	return refund, tax
}
