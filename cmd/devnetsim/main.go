package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
)

const (
	defaultPrepareValue      = int64(300_000)
	defaultSpamValue         = int64(120_000)
	defaultFeeRate           = int64(10)
	defaultFanoutSize        = 200
	defaultConsolidateInputs = 6
)

type cliConfig struct {
	rpcServer       string
	rpcUser         string
	rpcPass         string
	mirrorRPCServer string
	mirrorRPCUser   string
	mirrorRPCPass   string
	network         string
	stateFile       string
	seedTag         string
}

type simulator struct {
	client       *rpcclient.Client
	mirrorClient *rpcclient.Client
	net          *chaincfg.Params
	stateFile    string
	wallet       *wallet
	ownsClient   bool
}

type spamStats struct {
	Attempted        int
	Accepted         int
	Rejected         int
	Mode             string
	LastRejectReason string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "devnetsim: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		printUsage()
		return fmt.Errorf("missing command")
	}

	switch os.Args[1] {
	case "miningaddr":
		return runMiningAddr(os.Args[2:])
	case "status":
		return runStatus(os.Args[2:])
	case "newaddr":
		return runNewAddr(os.Args[2:])
	case "prepare":
		return runPrepare(os.Args[2:])
	case "spam":
		return runSpam(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", os.Args[1])
	}
}

func defaultStateFile() string {
	return filepath.Join(".", "devnet-data", "devnetsim", "state.json")
}

func resolveNetwork(name string) (*chaincfg.Params, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "simnet":
		return &chaincfg.SimNetParams, nil
	case "obtcregtest", "obtc-regtest":
		return &chaincfg.ObtcRegTestParams, nil
	default:
		return nil, fmt.Errorf("unsupported network %q", name)
	}
}

func addCommonFlags(fs *flag.FlagSet, cfg *cliConfig, includeRPC bool) {
	if includeRPC {
		fs.StringVar(&cfg.rpcServer, "rpcserver", "127.0.0.1:18556", "RPC server")
		fs.StringVar(&cfg.rpcUser, "rpcuser", "obtc", "RPC username")
		fs.StringVar(&cfg.rpcPass, "rpcpass", "obtcpass", "RPC password")
		fs.StringVar(&cfg.mirrorRPCServer, "mirror-rpcserver", "", "optional secondary RPC server for mirrored broadcast")
		fs.StringVar(&cfg.mirrorRPCUser, "mirror-rpcuser", "obtc", "secondary RPC username")
		fs.StringVar(&cfg.mirrorRPCPass, "mirror-rpcpass", "obtcpass", "secondary RPC password")
	}
	fs.StringVar(&cfg.network, "network", "simnet", "network: simnet|obtcregtest")
	fs.StringVar(&cfg.stateFile, "statefile", defaultStateFile(), "wallet state file")
	fs.StringVar(&cfg.seedTag, "seed-tag", "primary", "deterministic wallet namespace")
}

func newSimulator(cfg cliConfig) (*simulator, error) {
	net, err := resolveNetwork(cfg.network)
	if err != nil {
		return nil, err
	}

	connCfg := &rpcclient.ConnConfig{
		Host:         cfg.rpcServer,
		User:         cfg.rpcUser,
		Pass:         cfg.rpcPass,
		DisableTLS:   true,
		HTTPPostMode: true,
	}

	client, err := rpcclient.New(connCfg, nil)
	if err != nil {
		return nil, fmt.Errorf("connect rpc: %w", err)
	}

	sim, err := newSimulatorWithClient(client, cfg.stateFile, cfg.seedTag, net, true)
	if err != nil {
		client.Shutdown()
		client.WaitForShutdown()
		return nil, err
	}

	if cfg.mirrorRPCServer != "" {
		mirrorCfg := &rpcclient.ConnConfig{
			Host:         cfg.mirrorRPCServer,
			User:         cfg.mirrorRPCUser,
			Pass:         cfg.mirrorRPCPass,
			DisableTLS:   true,
			HTTPPostMode: true,
		}
		mirrorClient, err := rpcclient.New(mirrorCfg, nil)
		if err != nil {
			sim.close()
			return nil, fmt.Errorf("connect mirror rpc: %w", err)
		}
		sim.mirrorClient = mirrorClient
	}

	return sim, nil
}

func newSimulatorWithClient(client *rpcclient.Client, stateFile, seedTag string,
	net *chaincfg.Params, ownsClient bool) (*simulator, error) {

	km, err := loadKeyManager(stateFile, net, seedTag)
	if err != nil {
		return nil, err
	}

	return &simulator{
		client:     client,
		net:        net,
		stateFile:  stateFile,
		wallet:     newWallet(km),
		ownsClient: ownsClient,
	}, nil
}

func (s *simulator) close() {
	if s.mirrorClient != nil {
		s.mirrorClient.Shutdown()
		s.mirrorClient.WaitForShutdown()
	}
	if s.client != nil && s.ownsClient {
		s.client.Shutdown()
		s.client.WaitForShutdown()
	}
}

func (s *simulator) saveState() error {
	return s.wallet.km.save(s.stateFile)
}

func (s *simulator) syncWallet(target *wallet) error {
	bestHeight, err := s.client.GetBlockCount()
	if err != nil {
		return fmt.Errorf("get block count: %w", err)
	}

	target.resetConfirmed(int32(bestHeight))
	for height := int64(0); height <= bestHeight; height++ {
		blockHash, err := s.client.GetBlockHash(height)
		if err != nil {
			return fmt.Errorf("get block hash at %d: %w", height, err)
		}

		block, err := s.client.GetBlock(blockHash)
		if err != nil {
			return fmt.Errorf("get block %s: %w", blockHash, err)
		}

		for _, tx := range block.Transactions {
			if err := target.addConfirmedFromBlock(tx, int32(height)); err != nil {
				return err
			}
		}
	}

	target.clearPending()
	mempoolTxs, err := s.client.GetRawMempool()
	if err != nil {
		return fmt.Errorf("get raw mempool: %w", err)
	}
	for _, txHash := range mempoolTxs {
		tx, err := s.client.GetRawTransaction(txHash)
		if err != nil {
			return fmt.Errorf("get raw transaction %s: %w", txHash, err)
		}
		target.applyBroadcast(tx.MsgTx())
	}

	return nil
}

func (s *simulator) syncChain() error {
	return s.syncWallet(s.wallet)
}

func (s *simulator) mempoolCount() (int, error) {
	hashes, err := s.client.GetRawMempool()
	if err != nil {
		return 0, err
	}
	return len(hashes), nil
}

func (s *simulator) mineBlocks(num uint32) ([]*chainhash.Hash, error) {
	if num == 0 {
		return nil, nil
	}

	isGenerating, err := s.client.GetGenerate()
	if err == nil && isGenerating {
		if err := s.client.SetGenerate(false, -1); err != nil {
			return nil, fmt.Errorf("stop continuous miner: %w", err)
		}
	}

	hashes, err := s.client.Generate(num)
	if err != nil {
		return nil, err
	}

	s.wallet.clearPending()
	return hashes, nil
}

func (s *simulator) ensureSpendableBalance(required btcutil.Amount) error {
	if err := s.syncChain(); err != nil {
		return err
	}

	for s.wallet.spendableBalance() < required {
		blocks := uint32(25)
		if s.wallet.currentHeight < int32(s.net.CoinbaseMaturity) {
			blocks = uint32(s.net.CoinbaseMaturity) + 1
		}

		if _, err := s.mineBlocks(blocks); err != nil {
			return fmt.Errorf("mine funding blocks: %w", err)
		}
		if err := s.syncChain(); err != nil {
			return err
		}
	}

	return nil
}

func (s *simulator) sendRawTx(tx *wire.MsgTx) (string, error) {
	txHash, err := s.client.SendRawTransaction(tx, true)
	if err != nil {
		return "", err
	}

	if s.mirrorClient != nil {
		if _, err := s.mirrorClient.SendRawTransaction(tx, true); err != nil && !isIgnorableMirrorErr(err) {
			return "", fmt.Errorf("mirror broadcast: %w", err)
		}
	}

	s.wallet.applyBroadcast(tx)
	return txHash.String(), nil
}

func (s *simulator) prepareUTXOs(target int, outputValue, feeRate btcutil.Amount,
	fanoutSize int) error {
	return s.prepareWalletUTXOs(target, outputValue, feeRate, fanoutSize, s.wallet)
}

func (s *simulator) prepareWalletUTXOs(target int, outputValue, feeRate btcutil.Amount,
	fanoutSize int, recipient *wallet) error {
	if target <= 0 {
		return nil
	}
	if recipient == nil {
		return fmt.Errorf("recipient wallet is nil")
	}
	if fanoutSize <= 0 {
		fanoutSize = defaultFanoutSize
	}

	if err := s.syncChain(); err != nil {
		return err
	}
	if recipient != s.wallet {
		if err := s.syncWallet(recipient); err != nil {
			return err
		}
	}
	if recipient.spendableCount() >= target {
		return nil
	}

	margin := btcutil.Amount(target/fanoutSize+2) * 20_000
	required := btcutil.Amount(target)*outputValue + margin
	if err := s.ensureSpendableBalance(required); err != nil {
		return err
	}
	if err := s.syncChain(); err != nil {
		return err
	}
	if recipient != s.wallet {
		if err := s.syncWallet(recipient); err != nil {
			return err
		}
	}

	missing := target - recipient.spendableCount()
	for missing > 0 {
		batch := fanoutSize
		if batch > missing {
			batch = missing
		}

		var tx *wire.MsgTx
		var err error
		if recipient == s.wallet {
			amounts := make([]btcutil.Amount, batch)
			for i := range amounts {
				amounts[i] = outputValue
			}
			tx, err = s.wallet.createOutputsTx(amounts, feeRate, selectLargestConfirmed, true)
		} else {
			addresses, addrErr := recipient.km.newAddresses(batch)
			if addrErr != nil {
				return fmt.Errorf("allocate recipient addresses: %w", addrErr)
			}
			outputs := make([]paymentOutput, 0, len(addresses))
			for _, addr := range addresses {
				outputs = append(outputs, paymentOutput{Address: addr, Amount: outputValue})
			}
			tx, err = s.wallet.createPaymentTx(outputs, feeRate, selectLargestConfirmed, true)
		}
		if err != nil {
			return fmt.Errorf("build fanout tx: %w", err)
		}

		if _, err := s.sendRawTx(tx); err != nil {
			return fmt.Errorf("broadcast fanout tx: %w", err)
		}

		missing -= batch
	}

	if _, err := s.mineBlocks(1); err != nil {
		return fmt.Errorf("confirm fanout txs: %w", err)
	}
	if err := s.syncChain(); err != nil {
		return err
	}
	if recipient != s.wallet {
		if err := s.syncWallet(recipient); err != nil {
			return err
		}
	}

	return nil
}

func isIgnorableMirrorErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already have transaction") ||
		strings.Contains(msg, "already exists") ||
		strings.Contains(msg, "already in the mempool")
}

func feeRateForAttempt(baseFee btcutil.Amount, attempt int) btcutil.Amount {
	if baseFee <= 0 {
		baseFee = btcutil.Amount(defaultFeeRate)
	}

	multipliers := []int64{1, 1, 2, 2, 3, 5, 8, 13}
	multiplier := multipliers[attempt%len(multipliers)]
	adjusted := baseFee * btcutil.Amount(multiplier)
	if adjusted < 1 {
		return 1
	}
	return adjusted
}

func (s *simulator) buildConflictPair(feeRate btcutil.Amount) (*wire.MsgTx, *wire.MsgTx, error) {
	selected, err := s.wallet.selectUTXOs(selectSmallestConfirmed, false, 1)
	if err != nil {
		return nil, nil, err
	}

	primary, err := s.wallet.createSweepFromUTXOs(selected, feeRate)
	if err != nil {
		return nil, nil, err
	}

	conflictFee := feeRate * 2
	if conflictFee <= feeRate {
		conflictFee = feeRate + 1
	}
	conflict, err := s.wallet.createSweepFromUTXOs(selected, conflictFee)
	if err != nil {
		return nil, nil, err
	}

	return primary, conflict, nil
}

func (s *simulator) spam(count int, value, feeRate btcutil.Amount, mode string,
	pace time.Duration) (spamStats, error) {
	if err := s.syncChain(); err != nil {
		return spamStats{}, err
	}

	stats := spamStats{Mode: strings.ToLower(mode)}
	if count <= 0 {
		return stats, nil
	}

	for i := 0; i < count; i++ {
		stats.Attempted++

		var (
			tx  *wire.MsgTx
			err error
		)

		switch stats.Mode {
		case "simple":
			tx, err = s.wallet.createOutputsTx(
				[]btcutil.Amount{value}, feeRate, selectSmallestConfirmed, false,
			)

		case "mixed":
			switch {
			case i > 0 && i%17 == 0:
				tx, err = s.wallet.createSelfTransferTx(feeRate, true)
			case i > 0 && i%9 == 0:
				tx, err = s.wallet.createOutputsTx(
					splitValue(value, 3), feeRate, selectSmallestConfirmed, false,
				)
			case i > 0 && i%4 == 0:
				tx, err = s.wallet.createOutputsTx(
					splitValue(value, 2), feeRate, selectSmallestConfirmed, false,
				)
			default:
				tx, err = s.wallet.createOutputsTx(
					[]btcutil.Amount{value}, feeRate, selectSmallestConfirmed, false,
				)
			}

		case "chain":
			tx, err = s.wallet.createSelfTransferTx(feeRate, true)

		case "consolidate":
			tx, err = s.wallet.createSweepTx(
				defaultConsolidateInputs, feeRate,
				selectSmallestConfirmed, false,
			)

		case "feemarket":
			currentFee := feeRateForAttempt(feeRate, i)
			switch {
			case i > 0 && i%11 == 0:
				tx, err = s.wallet.createSweepTx(
					3, currentFee, selectSmallestConfirmed, false,
				)
			case i > 0 && i%6 == 0:
				tx, err = s.wallet.createOutputsTx(
					splitValue(value, 2), currentFee,
					selectSmallestConfirmed, false,
				)
			default:
				tx, err = s.wallet.createOutputsTx(
					[]btcutil.Amount{value}, currentFee,
					selectSmallestConfirmed, false,
				)
			}

		case "conflict":
			primary, conflict, err := s.buildConflictPair(feeRate)
			if err != nil {
				return stats, err
			}

			if _, err := s.sendRawTx(primary); err != nil {
				return stats, fmt.Errorf("broadcast primary conflict tx: %w", err)
			}
			stats.Accepted++

			if _, err := s.client.SendRawTransaction(conflict, true); err != nil {
				stats.Rejected++
				if stats.LastRejectReason == "" {
					stats.LastRejectReason = err.Error()
				}
			} else {
				s.wallet.applyBroadcast(conflict)
				stats.Accepted++
			}

			if pace > 0 {
				time.Sleep(pace)
			}
			continue

		default:
			return stats, fmt.Errorf("unsupported spam mode %q", mode)
		}
		if err != nil {
			return stats, err
		}

		if _, err := s.sendRawTx(tx); err != nil {
			return stats, err
		}
		stats.Accepted++

		if pace > 0 {
			time.Sleep(pace)
		}
	}

	return stats, nil
}

func runMiningAddr(args []string) error {
	var cfg cliConfig
	fs := flag.NewFlagSet("miningaddr", flag.ContinueOnError)
	addCommonFlags(fs, &cfg, false)
	if err := fs.Parse(args); err != nil {
		return err
	}

	net, err := resolveNetwork(cfg.network)
	if err != nil {
		return err
	}

	km, err := loadKeyManager(cfg.stateFile, net, cfg.seedTag)
	if err != nil {
		return err
	}

	addr, err := km.miningAddress()
	if err != nil {
		return err
	}

	fmt.Println(addr.EncodeAddress())
	return nil
}

func runNewAddr(args []string) error {
	var cfg cliConfig
	fs := flag.NewFlagSet("newaddr", flag.ContinueOnError)
	addCommonFlags(fs, &cfg, false)
	if err := fs.Parse(args); err != nil {
		return err
	}

	net, err := resolveNetwork(cfg.network)
	if err != nil {
		return err
	}

	km, err := loadKeyManager(cfg.stateFile, net, cfg.seedTag)
	if err != nil {
		return err
	}

	addr, _, err := km.newAddress()
	if err != nil {
		return err
	}
	if err := km.save(cfg.stateFile); err != nil {
		return err
	}

	fmt.Println(addr.EncodeAddress())
	return nil
}

func runStatus(args []string) error {
	var cfg cliConfig
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	addCommonFlags(fs, &cfg, true)
	if err := fs.Parse(args); err != nil {
		return err
	}

	sim, err := newSimulator(cfg)
	if err != nil {
		return err
	}
	defer sim.close()
	defer saveStateQuietly(sim)

	if err := sim.syncChain(); err != nil {
		return err
	}

	addr, err := sim.wallet.km.miningAddress()
	if err != nil {
		return err
	}

	mempoolCount, err := sim.mempoolCount()
	if err != nil {
		return err
	}

	fmt.Printf("network=%s\n", sim.net.Name)
	fmt.Printf("height=%d\n", sim.wallet.currentHeight)
	fmt.Printf("mining_address=%s\n", addr.EncodeAddress())
	fmt.Printf("next_index=%d\n", sim.wallet.km.nextIndex)
	fmt.Printf("confirmed_utxos=%d\n", sim.wallet.totalConfirmedCount())
	fmt.Printf("spendable_utxos=%d\n", sim.wallet.spendableCount())
	fmt.Printf("spendable_balance_sat=%d\n", int64(sim.wallet.spendableBalance()))
	fmt.Printf("mempool_txs=%d\n", mempoolCount)
	return nil
}

func runPrepare(args []string) error {
	var cfg cliConfig
	var (
		targetUTXOs        int
		valueSat           int64
		feeRateSat         int64
		fanoutSize         int
		recipientStateFile string
		recipientSeedTag   string
	)

	fs := flag.NewFlagSet("prepare", flag.ContinueOnError)
	addCommonFlags(fs, &cfg, true)
	fs.IntVar(&targetUTXOs, "utxos", 512, "target spendable UTXO count")
	fs.Int64Var(&valueSat, "value", defaultPrepareValue, "fanout output value in sat")
	fs.Int64Var(&feeRateSat, "fee-rate", defaultFeeRate, "fee rate in sat/vB")
	fs.IntVar(&fanoutSize, "fanout-size", defaultFanoutSize, "outputs per fanout transaction")
	fs.StringVar(&recipientStateFile, "recipient-statefile", "", "optional external recipient wallet state file")
	fs.StringVar(&recipientSeedTag, "recipient-seed-tag", "", "deterministic namespace for the recipient wallet")
	if err := fs.Parse(args); err != nil {
		return err
	}

	sim, err := newSimulator(cfg)
	if err != nil {
		return err
	}
	defer sim.close()
	defer saveStateQuietly(sim)

	recipientWallet := sim.wallet
	if recipientStateFile != "" {
		net, err := resolveNetwork(cfg.network)
		if err != nil {
			return err
		}
		recipientKM, err := loadKeyManager(recipientStateFile, net, recipientSeedTag)
		if err != nil {
			return err
		}
		recipientWallet = newWallet(recipientKM)
		defer func() {
			if err := recipientKM.save(recipientStateFile); err != nil {
				fmt.Fprintf(os.Stderr, "devnetsim: save recipient state: %v\n", err)
			}
		}()
	}

	if err := sim.prepareWalletUTXOs(targetUTXOs, btcutil.Amount(valueSat),
		btcutil.Amount(feeRateSat), fanoutSize, recipientWallet); err != nil {
		return err
	}

	mempoolCount, err := sim.mempoolCount()
	if err != nil {
		return err
	}

	fmt.Printf("prepared_utxos=%d\n", recipientWallet.spendableCount())
	fmt.Printf("prepared_balance_sat=%d\n", int64(recipientWallet.spendableBalance()))
	fmt.Printf("mempool_txs=%d\n", mempoolCount)
	if recipientStateFile != "" {
		fmt.Printf("recipient_statefile=%s\n", recipientStateFile)
	}
	return nil
}

func runSpam(args []string) error {
	var cfg cliConfig
	var (
		count        int
		valueSat     int64
		feeRateSat   int64
		mode         string
		prepareUTXOs int
		prepareValue int64
		fanoutSize   int
		paceMs       int
	)

	fs := flag.NewFlagSet("spam", flag.ContinueOnError)
	addCommonFlags(fs, &cfg, true)
	fs.IntVar(&count, "count", 100, "number of transactions to inject")
	fs.Int64Var(&valueSat, "value", defaultSpamValue, "total recipient value per transaction in sat")
	fs.Int64Var(&feeRateSat, "fee-rate", defaultFeeRate, "fee rate in sat/vB")
	fs.StringVar(&mode, "mode", "simple", "traffic mode: simple|mixed|chain|consolidate|feemarket|conflict")
	fs.IntVar(&prepareUTXOs, "prepare", 0, "ensure this many spendable UTXOs before spamming")
	fs.Int64Var(&prepareValue, "prepare-value", defaultPrepareValue, "prepared UTXO value in sat")
	fs.IntVar(&fanoutSize, "fanout-size", defaultFanoutSize, "outputs per fanout transaction")
	fs.IntVar(&paceMs, "pace-ms", 0, "sleep between broadcasts in milliseconds")
	if err := fs.Parse(args); err != nil {
		return err
	}

	sim, err := newSimulator(cfg)
	if err != nil {
		return err
	}
	defer sim.close()
	defer saveStateQuietly(sim)

	if prepareUTXOs > 0 {
		if err := sim.prepareUTXOs(
			prepareUTXOs, btcutil.Amount(prepareValue),
			btcutil.Amount(feeRateSat), fanoutSize,
		); err != nil {
			return err
		}
	}

	stats, err := sim.spam(
		count, btcutil.Amount(valueSat), btcutil.Amount(feeRateSat),
		mode, time.Duration(paceMs)*time.Millisecond,
	)
	if err != nil {
		return fmt.Errorf("accepted=%d rejected=%d: %w",
			stats.Accepted, stats.Rejected, err)
	}

	mempoolCount, err := sim.mempoolCount()
	if err != nil {
		return err
	}

	fmt.Printf("attempted=%d\n", stats.Attempted)
	fmt.Printf("accepted=%d\n", stats.Accepted)
	fmt.Printf("rejected=%d\n", stats.Rejected)
	fmt.Printf("mode=%s\n", strings.ToLower(mode))
	if stats.LastRejectReason != "" {
		reason := strings.ReplaceAll(stats.LastRejectReason, "\n", " ")
		fmt.Printf("last_reject_reason=%s\n", reason)
	}
	fmt.Printf("mempool_txs=%d\n", mempoolCount)
	return nil
}

func saveStateQuietly(sim *simulator) {
	if err := sim.saveState(); err != nil {
		fmt.Fprintf(os.Stderr, "devnetsim: save state: %v\n", err)
	}
}

func printUsage() {
	fmt.Println("Usage: devnetsim <command> [flags]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  miningaddr  Print the deterministic simnet mining address")
	fmt.Println("  newaddr     Allocate and print a new deterministic wallet address")
	fmt.Println("  status      Show wallet and mempool state")
	fmt.Println("  prepare     Pre-build confirmed UTXOs for later traffic")
	fmt.Println("  spam        Inject batches of realistic transactions")
}
