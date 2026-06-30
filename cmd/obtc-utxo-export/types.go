// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import "time"

type utxoRow struct {
	TxID                  string `json:"txid"`
	Vout                  uint32 `json:"vout"`
	Outpoint              string `json:"outpoint"`
	AmountSat             int64  `json:"amount_sat"`
	CreateHeight          uint64 `json:"create_height"`
	ExpiryHeight          uint64 `json:"expiry_height"`
	BlocksToExpiry        int64  `json:"blocks_to_expiry"`
	SnapshotHeight        int32  `json:"snapshot_height"`
	SnapshotHash          string `json:"snapshot_hash"`
	IsCoinbase            bool   `json:"is_coinbase"`
	ScriptType            string `json:"script_type"`
	ScriptPubKeyLength    int    `json:"script_pubkey_length"`
	ScriptPubKeySHA256Hex string `json:"script_pubkey_sha256"`
}

type exportManifest struct {
	Network                      string    `json:"network"`
	Source                       string    `json:"source"`
	SnapshotHeight               int32     `json:"snapshot_height"`
	SnapshotHash                 string    `json:"snapshot_hash"`
	SnapshotStable               bool      `json:"snapshot_stable"`
	FinalHeight                  int32     `json:"final_height"`
	FinalHash                    string    `json:"final_hash"`
	ForkHeight                   int32     `json:"fork_height,omitempty"`
	ForkHash                     string    `json:"fork_hash,omitempty"`
	DBPath                       string    `json:"db_path,omitempty"`
	UTXOStateHash                string    `json:"utxo_state_hash,omitempty"`
	UTXOStateConsistent          bool      `json:"utxo_state_consistent,omitempty"`
	RowCount                     int64     `json:"row_count"`
	StatsTotalUTXOs              int       `json:"stats_total_utxos"`
	SumAmountSat                 int64     `json:"sum_amount_sat"`
	MissingAmountCount           int64     `json:"missing_amount_count"`
	DuplicateOutpointCount       int64     `json:"duplicate_outpoint_count"`
	OrderViolationCount          int64     `json:"order_violation_count"`
	SkippedImmatureCoinbaseCount int64     `json:"skipped_immature_coinbase_count,omitempty"`
	FirstExpiryHeight            uint64    `json:"first_expiry_height"`
	LastExpiryHeight             uint64    `json:"last_expiry_height"`
	StartHeight                  int32     `json:"start_height"`
	EndHeight                    int32     `json:"end_height"`
	PageSize                     int       `json:"page_size"`
	Pages                        int64     `json:"pages"`
	ExportStartedAt              time.Time `json:"export_started_at"`
	ExportFinishedAt             time.Time `json:"export_finished_at"`
	DurationSeconds              float64   `json:"duration_seconds"`
	SHA256                       string    `json:"sha256"`
	FileSHA256                   string    `json:"file_sha256"`
	OutputFile                   string    `json:"output_file"`
}

type reapPreviewInput struct {
	TxID         string `json:"txid"`
	Vout         uint32 `json:"vout"`
	AmountSat    int64  `json:"amount_sat"`
	ExpiryHeight uint64 `json:"expiry_height"`
	TaxSat       int64  `json:"tax_sat"`
	RefundSat    int64  `json:"refund_sat"`
	Dust         bool   `json:"dust"`
}

type reapPreviewDetail struct {
	ReapHeight         uint64             `json:"reap_height"`
	SelectedInputs     []reapPreviewInput `json:"selected_inputs"`
	SelectedInputCount int                `json:"selected_input_count"`
	TaxTotalSat        int64              `json:"tax_total_sat"`
	RefundTotalSat     int64              `json:"refund_total_sat"`
	DustTaxSat         int64              `json:"dust_tax_sat"`
	NormalTaxSat       int64              `json:"normal_tax_sat"`
	DustInputs         int                `json:"dust_inputs"`
	NormalInputs       int                `json:"normal_inputs"`
	EstWeight          int64              `json:"est_weight"`
	RemainingBacklog   int                `json:"remaining_backlog"`
}

type reapHeightSummary struct {
	ReapHeight       uint64 `json:"reap_height"`
	SelectedInputs   int    `json:"selected_inputs"`
	TaxTotalSat      int64  `json:"tax_total_sat"`
	RefundTotalSat   int64  `json:"refund_total_sat"`
	DustTaxSat       int64  `json:"dust_tax_sat"`
	NormalTaxSat     int64  `json:"normal_tax_sat"`
	DustInputs       int    `json:"dust_inputs"`
	NormalInputs     int    `json:"normal_inputs"`
	RemainingBacklog int    `json:"remaining_backlog"`
}

type reapBucketSummary struct {
	StartHeight    uint64 `json:"start_height"`
	EndHeight      uint64 `json:"end_height"`
	SelectedInputs int    `json:"selected_inputs"`
	TaxTotalSat    int64  `json:"tax_total_sat"`
	RefundTotalSat int64  `json:"refund_total_sat"`
	DustTaxSat     int64  `json:"dust_tax_sat"`
	NormalTaxSat   int64  `json:"normal_tax_sat"`
}

type reapPreviewSummary struct {
	Network             string              `json:"network"`
	SnapshotHeight      int32               `json:"snapshot_height"`
	SnapshotHash        string              `json:"snapshot_hash"`
	GeneratedAt         time.Time           `json:"generated_at"`
	UTXORowCount        int                 `json:"utxo_row_count"`
	ReapBlockCount      int                 `json:"reap_block_count"`
	SelectedInputs      int                 `json:"selected_inputs"`
	TaxTotalSat         int64               `json:"tax_total_sat"`
	RefundTotalSat      int64               `json:"refund_total_sat"`
	DustTaxSat          int64               `json:"dust_tax_sat"`
	NormalTaxSat        int64               `json:"normal_tax_sat"`
	MaxRemainingBacklog int                 `json:"max_remaining_backlog"`
	Params              reapPreviewParams   `json:"params"`
	ByHeight            []reapHeightSummary `json:"by_height"`
	ByDay               []reapBucketSummary `json:"by_day"`
	ByWeek              []reapBucketSummary `json:"by_week"`
	DetailSHA256        string              `json:"detail_sha256"`
	DetailFileSHA256    string              `json:"detail_file_sha256"`
}

type reapPreviewParams struct {
	TaxNumerator     int64 `json:"tax_numerator"`
	TaxDenominator   int64 `json:"tax_denominator"`
	DustThresholdSat int64 `json:"dust_threshold_sat"`
	MaxInputs        int   `json:"max_inputs"`
	DustMaxInputs    int   `json:"dust_max_inputs"`
	WeightBudget     int64 `json:"weight_budget"`
}

type reapAggregateBlock struct {
	ReapHeight       uint64 `json:"reap_height"`
	ExpiredInputs    int64  `json:"expired_inputs"`
	ExpiredAmountSat int64  `json:"expired_amount_sat"`
	SelectedInputs   int64  `json:"selected_inputs"`
	TaxTotalSat      int64  `json:"tax_total_sat"`
	RefundTotalSat   int64  `json:"refund_total_sat"`
	DustTaxSat       int64  `json:"dust_tax_sat"`
	NormalTaxSat     int64  `json:"normal_tax_sat"`
	DustInputs       int64  `json:"dust_inputs"`
	NormalInputs     int64  `json:"normal_inputs"`
	RemainingBacklog int64  `json:"remaining_backlog"`
}

type reapAggregateSummary struct {
	Network             string            `json:"network"`
	SnapshotHeight      int32             `json:"snapshot_height"`
	SnapshotHash        string            `json:"snapshot_hash"`
	GeneratedAt         time.Time         `json:"generated_at"`
	UTXORowCount        int64             `json:"utxo_row_count"`
	ReapBlockCount      int64             `json:"reap_block_count"`
	ReapStartHeight     uint64            `json:"reap_start_height"`
	FirstReapHeight     uint64            `json:"first_reap_height"`
	LastReapHeight      uint64            `json:"last_reap_height"`
	ExpiredInputs       int64             `json:"expired_inputs"`
	ExpiredAmountSat    int64             `json:"expired_amount_sat"`
	SelectedInputs      int64             `json:"selected_inputs"`
	TaxTotalSat         int64             `json:"tax_total_sat"`
	RefundTotalSat      int64             `json:"refund_total_sat"`
	DustTaxSat          int64             `json:"dust_tax_sat"`
	NormalTaxSat        int64             `json:"normal_tax_sat"`
	MaxRemainingBacklog int64             `json:"max_remaining_backlog"`
	Params              reapPreviewParams `json:"params"`
	BlocksSHA256        string            `json:"blocks_sha256"`
	BlocksFileSHA256    string            `json:"blocks_file_sha256"`
	BlocksFile          string            `json:"blocks_file"`
	ShardSpan           uint64            `json:"shard_span"`
}
