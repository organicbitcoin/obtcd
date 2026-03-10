package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	flags "github.com/jessevdk/go-flags"
)

var (
	btcdHomeDir        = btcutil.AppDataDir("btcd", false)
	defaultRPCCertFile = filepath.Join(btcdHomeDir, "rpc.cert")
)

type config struct {
	Listen        string        `long:"listen" description:"HTTP listen address for the read-only status page" default:"127.0.0.1:9680"`
	Refresh       time.Duration `long:"refresh" description:"Auto-refresh interval for the HTML page" default:"15s"`
	RPCTimeout    time.Duration `long:"rpctimeout" description:"Timeout for upstream btcd RPC calls" default:"5s"`
	NoTLS         bool          `long:"notls" description:"Disable TLS for upstream btcd RPC calls"`
	ObtcMainNet   bool          `long:"obtcmainnet" description:"Connect to the OBTC main network"`
	ObtcTestNet   bool          `long:"obtctestnet" description:"Connect to the OBTC test network"`
	ObtcRegTest   bool          `long:"obtcregtest" description:"Connect to the OBTC regression test network"`
	RPCCert       string        `short:"c" long:"rpccert" description:"RPC server certificate chain for validation"`
	RPCPassword   string        `short:"P" long:"rpcpass" default-mask:"-" description:"RPC password"`
	RPCServer     string        `short:"s" long:"rpcserver" description:"RPC server to connect to" default:"127.0.0.1"`
	RPCUser       string        `short:"u" long:"rpcuser" description:"RPC username"`
	ShowVersion   bool          `short:"V" long:"version" description:"Display version information and exit"`
	TLSSkipVerify bool          `long:"skipverify" description:"Do not verify TLS certificates (not recommended)"`
}

func normalizeAddress(addr, defaultPort string) (string, error) {
	_, _, err := net.SplitHostPort(addr)
	if err == nil {
		return addr, nil
	}

	hostErr, ok := err.(*net.AddrError)
	if ok && hostErr.Err == "missing port in address" {
		return net.JoinHostPort(addr, defaultPort), nil
	}

	return "", err
}

func cleanAndExpandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		homeDir := filepath.Dir(btcdHomeDir)
		path = strings.Replace(path, "~", homeDir, 1)
	}

	return filepath.Clean(os.ExpandEnv(path))
}

func networkParams(cfg *config) (*chaincfg.Params, error) {
	network := &chaincfg.ObtcMainNetParams
	numNets := 0

	if cfg.ObtcMainNet {
		numNets++
		network = &chaincfg.ObtcMainNetParams
	}
	if cfg.ObtcTestNet {
		numNets++
		network = &chaincfg.ObtcTestNetParams
	}
	if cfg.ObtcRegTest {
		numNets++
		network = &chaincfg.ObtcRegTestParams
	}
	if numNets > 1 {
		return nil, fmt.Errorf("multiple network params can't be used together -- choose one")
	}

	return network, nil
}

func defaultRPCPort(params *chaincfg.Params) string {
	switch params {
	case &chaincfg.ObtcTestNetParams:
		return "18556"
	case &chaincfg.ObtcRegTestParams:
		return "18667"
	default:
		return "8556"
	}
}

func loadConfig() (*config, error) {
	cfg := &config{
		RPCCert: defaultRPCCertFile,
	}

	parser := flags.NewParser(cfg, flags.HelpFlag)
	_, err := parser.Parse()
	if err != nil {
		return nil, err
	}

	if cfg.ShowVersion {
		fmt.Println("obtc-status version", version())
		os.Exit(0)
	}
	if cfg.Refresh <= 0 {
		return nil, fmt.Errorf("--refresh must be positive")
	}
	if cfg.RPCTimeout <= 0 {
		return nil, fmt.Errorf("--rpctimeout must be positive")
	}

	params, err := networkParams(cfg)
	if err != nil {
		return nil, err
	}

	cfg.RPCServer, err = normalizeAddress(cfg.RPCServer, defaultRPCPort(params))
	if err != nil {
		return nil, fmt.Errorf("invalid rpcserver: %w", err)
	}
	addr, err := net.ResolveTCPAddr("tcp", cfg.Listen)
	if err != nil || addr == nil {
		if err == nil {
			err = fmt.Errorf("invalid listen address")
		}
		return nil, err
	}

	cfg.RPCCert = cleanAndExpandPath(cfg.RPCCert)
	return cfg, nil
}
