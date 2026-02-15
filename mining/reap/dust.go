package reap

// applyDustRule folds tiny refund amounts back into tax for a draft REAP dust rule.
// If dustThresholdSat <= 0, the rule is disabled.
func applyDustRule(refund, tax, dustThresholdSat int64) (adjRefund, adjTax int64) {
	if dustThresholdSat <= 0 {
		return refund, tax
	}
	if refund > 0 && refund < dustThresholdSat {
		return 0, tax + refund
	}
	return refund, tax
}
