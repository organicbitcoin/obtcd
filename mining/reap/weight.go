// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package reap

const (
	estBaseWeight         int64 = 40
	estInputWeight        int64 = 164
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

// EstimateTieredBlueprintWeight returns a conservative estimate for REAP
// blueprints under the two-tier cap rules.  Dust inputs are refundless by
// definition, so only normal inputs are modeled with refund outputs.
func EstimateTieredBlueprintWeight(dustInputs, normalInputs int) int64 {
	if dustInputs < 0 {
		dustInputs = 0
	}
	if normalInputs < 0 {
		normalInputs = 0
	}
	totalInputs := dustInputs + normalInputs
	return estBaseWeight + int64(totalInputs)*estInputWeight +
		int64(normalInputs)*estRefundOutputWeight + estMarkOutputWeight
}
