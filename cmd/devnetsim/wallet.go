// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
)

const (
	defaultDustThreshold = btcutil.Amount(1_000)

	// P2PKH script max size used to estimate fee before signatures are added.
	spendSize = 1 + 73 + 1 + 33
)

var defaultHDSeed = [32]byte{
	0x79, 0xa6, 0x1a, 0xdb, 0xc6, 0xe5, 0xa2, 0xe1,
	0x39, 0xd2, 0x71, 0x3a, 0x54, 0x6e, 0xc7, 0xc8,
	0x75, 0x63, 0x2e, 0x75, 0xf1, 0xdf, 0x9c, 0x3f,
	0xa6, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
}

func deriveSeed(seedTag string) [32]byte {
	if seedTag == "" || seedTag == "primary" {
		return defaultHDSeed
	}

	h := sha256.New()
	h.Write(defaultHDSeed[:])
	h.Write([]byte("devnetsim:"))
	h.Write([]byte(seedTag))

	var derived [32]byte
	copy(derived[:], h.Sum(nil))
	return derived
}

type persistedState struct {
	NextIndex uint32 `json:"next_index"`
}

type walletUTXO struct {
	OutPoint       wire.OutPoint
	PkScript       []byte
	Value          btcutil.Amount
	KeyIndex       uint32
	BlockHeight    int32
	MaturityHeight int32
	Confirmed      bool
}

type paymentOutput struct {
	Address btcutil.Address
	Amount  btcutil.Amount
}

func (u *walletUTXO) isExpired(currentHeight int32, net *chaincfg.Params) bool {
	if !u.Confirmed || net == nil {
		return false
	}

	expiryParams := chaincfg.GetExpiryParams(net)
	if expiryParams == nil {
		return false
	}

	spendHeight := currentHeight + 1
	if spendHeight < expiryParams.EnableAtHeight {
		return false
	}

	return spendHeight >= int32(expiryParams.CalculateExpiryKey(u.BlockHeight))
}

func (u *walletUTXO) isSpendable(currentHeight int32, net *chaincfg.Params) bool {
	if !u.Confirmed {
		return true
	}
	if currentHeight < u.MaturityHeight {
		return false
	}
	return !u.isExpired(currentHeight, net)
}

type selectionMode int

const (
	selectLargestConfirmed selectionMode = iota
	selectSmallestConfirmed
	selectPendingFirst
	selectRandomConfirmed
	selectRandomPendingFirst
)

type keyManager struct {
	net       *chaincfg.Params
	root      *hdkeychain.ExtendedKey
	nextIndex uint32
	addrs     map[uint32]btcutil.Address
	scripts   map[string]uint32
}

func loadKeyManager(statePath string, net *chaincfg.Params, seedTag string) (*keyManager, error) {
	seed := deriveSeed(seedTag)
	root, err := hdkeychain.NewMaster(seed[:], net)
	if err != nil {
		return nil, err
	}

	state := persistedState{NextIndex: 1}
	data, err := os.ReadFile(statePath)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &state); err != nil {
			return nil, fmt.Errorf("decode state file: %w", err)
		}
	case os.IsNotExist(err):
		// Fresh state.
	default:
		return nil, fmt.Errorf("read state file: %w", err)
	}

	if state.NextIndex == 0 {
		state.NextIndex = 1
	}

	km := &keyManager{
		net:       net,
		root:      root,
		nextIndex: state.NextIndex,
		addrs:     make(map[uint32]btcutil.Address),
		scripts:   make(map[string]uint32),
	}

	for i := uint32(0); i < km.nextIndex; i++ {
		if _, err := km.ensureAddress(i); err != nil {
			return nil, err
		}
	}

	return km, nil
}

func (km *keyManager) save(statePath string) error {
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		return err
	}

	payload, err := json.MarshalIndent(persistedState{
		NextIndex: km.nextIndex,
	}, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(statePath, payload, 0o600)
}

func (km *keyManager) ensureAddress(index uint32) (btcutil.Address, error) {
	if addr, ok := km.addrs[index]; ok {
		return addr, nil
	}

	childKey, err := km.root.Derive(index)
	if err != nil {
		return nil, err
	}

	privKey, err := childKey.ECPrivKey()
	if err != nil {
		return nil, err
	}

	addr, err := keyToAddr(privKey, km.net)
	if err != nil {
		return nil, err
	}

	pkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		return nil, err
	}

	km.addrs[index] = addr
	km.scripts[hex.EncodeToString(pkScript)] = index
	return addr, nil
}

func (km *keyManager) miningAddress() (btcutil.Address, error) {
	return km.ensureAddress(0)
}

func (km *keyManager) newAddress() (btcutil.Address, uint32, error) {
	index := km.nextIndex
	addr, err := km.ensureAddress(index)
	if err != nil {
		return nil, 0, err
	}
	km.nextIndex++
	return addr, index, nil
}

func (km *keyManager) newAddresses(count int) ([]btcutil.Address, error) {
	if count <= 0 {
		return nil, nil
	}

	addrs := make([]btcutil.Address, 0, count)
	for i := 0; i < count; i++ {
		addr, _, err := km.newAddress()
		if err != nil {
			return nil, err
		}
		addrs = append(addrs, addr)
	}

	return addrs, nil
}

func (km *keyManager) privateKey(index uint32) (*btcec.PrivateKey, error) {
	childKey, err := km.root.Derive(index)
	if err != nil {
		return nil, err
	}
	return childKey.ECPrivKey()
}

func (km *keyManager) matchScript(pkScript []byte) (uint32, bool) {
	keyIndex, ok := km.scripts[hex.EncodeToString(pkScript)]
	return keyIndex, ok
}

type wallet struct {
	km            *keyManager
	currentHeight int32
	confirmed     map[wire.OutPoint]*walletUTXO
	pending       map[wire.OutPoint]*walletUTXO
	pendingSpends map[wire.OutPoint]struct{}
	rng           *rand.Rand
}

func newWallet(km *keyManager) *wallet {
	return &wallet{
		km:            km,
		confirmed:     make(map[wire.OutPoint]*walletUTXO),
		pending:       make(map[wire.OutPoint]*walletUTXO),
		pendingSpends: make(map[wire.OutPoint]struct{}),
		rng:           rand.New(rand.NewSource(1)),
	}
}

func (w *wallet) setRandSeed(seed int64) {
	w.rng = rand.New(rand.NewSource(seed))
}

func (w *wallet) resetConfirmed(height int32) {
	w.currentHeight = height
	w.confirmed = make(map[wire.OutPoint]*walletUTXO)
}

func (w *wallet) clearPending() {
	w.pending = make(map[wire.OutPoint]*walletUTXO)
	w.pendingSpends = make(map[wire.OutPoint]struct{})
}

func (w *wallet) addConfirmedFromBlock(tx *wire.MsgTx, blockHeight int32) error {
	isCoinbase := blockchain.IsCoinBaseTx(tx)
	txHash := tx.TxHash()

	for i, txOut := range tx.TxOut {
		keyIndex, ok := w.km.matchScript(txOut.PkScript)
		if !ok {
			continue
		}

		maturityHeight := int32(0)
		if isCoinbase {
			maturityHeight = blockHeight + int32(w.km.net.CoinbaseMaturity)
		}

		op := wire.OutPoint{Hash: txHash, Index: uint32(i)}
		w.confirmed[op] = &walletUTXO{
			OutPoint:       op,
			PkScript:       txOut.PkScript,
			Value:          btcutil.Amount(txOut.Value),
			KeyIndex:       keyIndex,
			BlockHeight:    blockHeight,
			MaturityHeight: maturityHeight,
			Confirmed:      true,
		}
	}

	if !isCoinbase {
		for _, txIn := range tx.TxIn {
			delete(w.confirmed, txIn.PreviousOutPoint)
		}
	}

	return nil
}

func (w *wallet) spendableBalance() btcutil.Amount {
	var total btcutil.Amount
	for _, utxo := range w.confirmed {
		if utxo.isSpendable(w.currentHeight, w.km.net) {
			total += utxo.Value
		}
	}
	return total
}

func (w *wallet) spendableCount() int {
	count := 0
	for _, utxo := range w.confirmed {
		if utxo.isSpendable(w.currentHeight, w.km.net) {
			count++
		}
	}
	return count
}

func (w *wallet) totalConfirmedCount() int {
	return len(w.confirmed)
}

func (w *wallet) utxoForInput(op wire.OutPoint) (*walletUTXO, error) {
	if utxo, ok := w.pending[op]; ok {
		return utxo, nil
	}
	if utxo, ok := w.confirmed[op]; ok {
		return utxo, nil
	}
	return nil, fmt.Errorf("unknown wallet input %s:%d", op.Hash, op.Index)
}

type candidateUTXO struct {
	utxo      *walletUTXO
	isPending bool
}

func (w *wallet) candidates(mode selectionMode, allowPending bool) []candidateUTXO {
	candidates := make([]candidateUTXO, 0, len(w.confirmed)+len(w.pending))

	for _, utxo := range w.confirmed {
		if !utxo.isSpendable(w.currentHeight, w.km.net) {
			continue
		}
		candidates = append(candidates, candidateUTXO{
			utxo:      utxo,
			isPending: false,
		})
	}

	if allowPending {
		for _, utxo := range w.pending {
			candidates = append(candidates, candidateUTXO{
				utxo:      utxo,
				isPending: true,
			})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]

		switch mode {
		case selectRandomPendingFirst:
			if left.isPending != right.isPending {
				return left.isPending
			}
		case selectRandomConfirmed:
			// Leave the slice in insertion order before shuffling below.
		case selectPendingFirst:
			if left.isPending != right.isPending {
				return left.isPending
			}
			if left.utxo.Value != right.utxo.Value {
				return left.utxo.Value > right.utxo.Value
			}
		case selectSmallestConfirmed:
			if left.utxo.Value != right.utxo.Value {
				return left.utxo.Value < right.utxo.Value
			}
		default:
			if left.utxo.Value != right.utxo.Value {
				return left.utxo.Value > right.utxo.Value
			}
		}

		if left.utxo.OutPoint.Hash != right.utxo.OutPoint.Hash {
			return left.utxo.OutPoint.Hash.String() < right.utxo.OutPoint.Hash.String()
		}

		return left.utxo.OutPoint.Index < right.utxo.OutPoint.Index
	})

	switch mode {
	case selectRandomConfirmed:
		w.shuffleCandidates(candidates)
	case selectRandomPendingFirst:
		pendingCount := 0
		for _, candidate := range candidates {
			if !candidate.isPending {
				break
			}
			pendingCount++
		}
		w.shuffleCandidates(candidates[:pendingCount])
		w.shuffleCandidates(candidates[pendingCount:])
	}

	return candidates
}

func (w *wallet) shuffleCandidates(candidates []candidateUTXO) {
	if len(candidates) < 2 || w.rng == nil {
		return
	}
	w.rng.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})
}

func (w *wallet) selectUTXOs(mode selectionMode, allowPending bool, maxInputs int) ([]*walletUTXO, error) {
	if maxInputs <= 0 {
		maxInputs = 1
	}

	candidates := w.candidates(mode, allowPending)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no spendable utxo available")
	}
	if maxInputs > len(candidates) {
		maxInputs = len(candidates)
	}

	selected := make([]*walletUTXO, 0, maxInputs)
	for i := 0; i < maxInputs; i++ {
		selected = append(selected, candidates[i].utxo)
	}

	return selected, nil
}

func (w *wallet) createOutputsTx(amounts []btcutil.Amount, feeRate btcutil.Amount,
	mode selectionMode, allowPending bool) (*wire.MsgTx, error) {

	outputs := make([]paymentOutput, 0, len(amounts))
	for _, amount := range amounts {
		addr, _, err := w.km.newAddress()
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, paymentOutput{
			Address: addr,
			Amount:  amount,
		})
	}

	return w.createPaymentTx(outputs, feeRate, mode, allowPending)
}

func (w *wallet) createPaymentTx(outputs []paymentOutput, feeRate btcutil.Amount,
	mode selectionMode, allowPending bool) (*wire.MsgTx, error) {

	if len(outputs) == 0 {
		return nil, fmt.Errorf("no outputs requested")
	}

	tx := wire.NewMsgTx(wire.TxVersion)
	for _, output := range outputs {
		if output.Address == nil {
			return nil, fmt.Errorf("nil payment address")
		}
		if output.Amount <= defaultDustThreshold {
			return nil, fmt.Errorf("payment output %d is dust", output.Amount)
		}

		pkScript, err := txscript.PayToAddrScript(output.Address)
		if err != nil {
			return nil, err
		}

		tx.AddTxOut(&wire.TxOut{
			Value:    int64(output.Amount),
			PkScript: pkScript,
		})
	}

	if err := w.fundTx(tx, feeRate, mode, allowPending); err != nil {
		return nil, err
	}
	if err := w.signTx(tx); err != nil {
		return nil, err
	}

	return tx, nil
}

func (w *wallet) createSelfTransferTx(feeRate btcutil.Amount,
	mode selectionMode, allowPending bool) (*wire.MsgTx, error) {

	candidates := w.candidates(mode, allowPending)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no spendable utxo available for self-transfer")
	}

	selected := candidates[0].utxo
	tx := wire.NewMsgTx(wire.TxVersion)
	tx.AddTxIn(wire.NewTxIn(&selected.OutPoint, nil, nil))

	addr, _, err := w.km.newAddress()
	if err != nil {
		return nil, err
	}

	pkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		return nil, err
	}

	// Estimate fee for a one-input, one-output P2PKH transaction.
	estSize := tx.SerializeSize() + spendSize + len(pkScript) + 9
	fee := btcutil.Amount(estSize) * feeRate
	outputValue := selected.Value - fee
	if outputValue <= defaultDustThreshold {
		return nil, fmt.Errorf("selected utxo %s:%d too small for self-transfer",
			selected.OutPoint.Hash, selected.OutPoint.Index)
	}

	tx.AddTxOut(&wire.TxOut{
		Value:    int64(outputValue),
		PkScript: pkScript,
	})

	if err := w.signTx(tx); err != nil {
		return nil, err
	}

	return tx, nil
}

func (w *wallet) createSweepTx(inputCount int, feeRate btcutil.Amount,
	mode selectionMode, allowPending bool) (*wire.MsgTx, error) {

	utxos, err := w.selectUTXOs(mode, allowPending, inputCount)
	if err != nil {
		return nil, err
	}

	return w.createSweepFromUTXOs(utxos, feeRate)
}

func (w *wallet) createSweepFromUTXOs(utxos []*walletUTXO,
	feeRate btcutil.Amount) (*wire.MsgTx, error) {

	if len(utxos) == 0 {
		return nil, fmt.Errorf("no utxos selected for sweep")
	}

	tx := wire.NewMsgTx(wire.TxVersion)
	var totalInput btcutil.Amount
	for _, utxo := range utxos {
		if utxo == nil {
			return nil, fmt.Errorf("nil utxo selected for sweep")
		}

		tx.AddTxIn(wire.NewTxIn(&utxo.OutPoint, nil, nil))
		totalInput += utxo.Value
	}

	addr, _, err := w.km.newAddress()
	if err != nil {
		return nil, err
	}

	pkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		return nil, err
	}

	estSize := tx.SerializeSize() + spendSize*len(tx.TxIn) + len(pkScript) + 9
	fee := btcutil.Amount(estSize) * feeRate
	outputValue := totalInput - fee
	if outputValue <= defaultDustThreshold {
		return nil, fmt.Errorf("selected utxos total %d too small for sweep",
			totalInput)
	}

	tx.AddTxOut(&wire.TxOut{
		Value:    int64(outputValue),
		PkScript: pkScript,
	})

	if err := w.signTxWithUTXOs(tx, utxos); err != nil {
		return nil, err
	}

	return tx, nil
}

func (w *wallet) fundTx(tx *wire.MsgTx, feeRate btcutil.Amount,
	mode selectionMode, allowPending bool) error {

	var totalOutput btcutil.Amount
	for _, txOut := range tx.TxOut {
		totalOutput += btcutil.Amount(txOut.Value)
	}

	candidates := w.candidates(mode, allowPending)
	var selectedTotal btcutil.Amount

	for _, candidate := range candidates {
		tx.AddTxIn(wire.NewTxIn(&candidate.utxo.OutPoint, nil, nil))
		selectedTotal += candidate.utxo.Value

		estSize := tx.SerializeSize() + spendSize*len(tx.TxIn)
		requiredFee := btcutil.Amount(estSize) * feeRate
		if selectedTotal < totalOutput+requiredFee {
			continue
		}

		changeValue := selectedTotal - totalOutput - requiredFee
		if changeValue > defaultDustThreshold {
			addr, _, err := w.km.newAddress()
			if err != nil {
				return err
			}
			changeScript, err := txscript.PayToAddrScript(addr)
			if err != nil {
				return err
			}
			tx.AddTxOut(&wire.TxOut{
				Value:    int64(changeValue),
				PkScript: changeScript,
			})

			estSize = tx.SerializeSize() + spendSize*len(tx.TxIn)
			requiredFee = btcutil.Amount(estSize) * feeRate
			changeValue = selectedTotal - totalOutput - requiredFee
			if changeValue > defaultDustThreshold {
				tx.TxOut[len(tx.TxOut)-1].Value = int64(changeValue)
				return nil
			}

			tx.TxOut = tx.TxOut[:len(tx.TxOut)-1]
		}

		return nil
	}

	return fmt.Errorf("not enough wallet funds for %d outputs", len(tx.TxOut))
}

func (w *wallet) signatureHashType() txscript.SigHashType {
	hashType := txscript.SigHashAll
	if chaincfg.IsOBTCReplayProtectionActive(w.km.net, w.currentHeight+1) {
		hashType |= txscript.SigHashOBTCReplayProtection
	}
	return hashType
}

func (w *wallet) signTx(tx *wire.MsgTx) error {
	hashType := w.signatureHashType()
	for i, txIn := range tx.TxIn {
		utxo, err := w.utxoForInput(txIn.PreviousOutPoint)
		if err != nil {
			return err
		}

		privKey, err := w.km.privateKey(utxo.KeyIndex)
		if err != nil {
			return err
		}

		sigScript, err := txscript.SignatureScript(
			tx, i, utxo.PkScript, hashType, privKey, true,
		)
		if err != nil {
			return err
		}

		txIn.SignatureScript = sigScript
	}

	return nil
}

func (w *wallet) signTxWithUTXOs(tx *wire.MsgTx, utxos []*walletUTXO) error {
	hashType := w.signatureHashType()
	lookup := make(map[wire.OutPoint]*walletUTXO, len(utxos))
	for _, utxo := range utxos {
		if utxo == nil {
			continue
		}
		lookup[utxo.OutPoint] = utxo
	}

	for i, txIn := range tx.TxIn {
		utxo, ok := lookup[txIn.PreviousOutPoint]
		if !ok {
			return fmt.Errorf("missing selected utxo for input %s:%d",
				txIn.PreviousOutPoint.Hash, txIn.PreviousOutPoint.Index)
		}

		privKey, err := w.km.privateKey(utxo.KeyIndex)
		if err != nil {
			return err
		}

		sigScript, err := txscript.SignatureScript(
			tx, i, utxo.PkScript, hashType, privKey, true,
		)
		if err != nil {
			return err
		}

		txIn.SignatureScript = sigScript
	}

	return nil
}

func (w *wallet) applyBroadcast(tx *wire.MsgTx) {
	for _, txIn := range tx.TxIn {
		delete(w.confirmed, txIn.PreviousOutPoint)
		delete(w.pending, txIn.PreviousOutPoint)
		w.pendingSpends[txIn.PreviousOutPoint] = struct{}{}
	}

	txHash := tx.TxHash()
	for i, txOut := range tx.TxOut {
		keyIndex, ok := w.km.matchScript(txOut.PkScript)
		if !ok {
			continue
		}

		op := wire.OutPoint{Hash: txHash, Index: uint32(i)}
		w.pending[op] = &walletUTXO{
			OutPoint:       op,
			PkScript:       txOut.PkScript,
			Value:          btcutil.Amount(txOut.Value),
			KeyIndex:       keyIndex,
			BlockHeight:    w.currentHeight + 1,
			MaturityHeight: 0,
			Confirmed:      false,
		}
	}
}

func splitValue(total btcutil.Amount, outputs int) []btcutil.Amount {
	if outputs <= 1 {
		return []btcutil.Amount{total}
	}

	parts := make([]btcutil.Amount, outputs)
	base := total / btcutil.Amount(outputs)
	remainder := total % btcutil.Amount(outputs)
	for i := 0; i < outputs; i++ {
		parts[i] = base
		if remainder > 0 {
			parts[i]++
			remainder--
		}
	}
	return parts
}

func keyToAddr(key *btcec.PrivateKey, net *chaincfg.Params) (btcutil.Address, error) {
	serializedKey := key.PubKey().SerializeCompressed()
	pubKeyAddr, err := btcutil.NewAddressPubKey(serializedKey, net)
	if err != nil {
		return nil, err
	}
	return pubKeyAddr.AddressPubKeyHash(), nil
}
