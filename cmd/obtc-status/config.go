// Copyright (c) 2026 The OBTC developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

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
	Listen              string        `long:"listen" description:"HTTP listen address for the read-only status page" default:"127.0.0.1:9680"`
	Refresh             time.Duration `long:"refresh" description:"Auto-refresh interval for the HTML page" default:"15s"`
	RPCTimeout          time.Duration `long:"rpctimeout" description:"Timeout for upstream btcd RPC calls" default:"5s"`
	Devnet              bool          `long:"devnet" description:"Serve the local Devnet dashboard instead of a single-node status page"`
	TestnetLab          bool          `long:"testnet-lab" description:"Serve the public testnet lab dashboard from a manifest"`
	LabManifest         string        `long:"lab-manifest" description:"Path to the public testnet lab manifest file" default:"./testnet-lab.json"`
	LabScript           string        `long:"lab-script" description:"Path to the local public testnet lab action script" default:"./scripts/testnet-lab.sh"`
	LabActionTimeout    time.Duration `long:"lab-action-timeout" description:"Timeout for local public testnet lab actions" default:"2m"`
	LabLogTailLines     int           `long:"lab-log-tail-lines" description:"Recent log lines scanned per lab log source" default:"400"`
	DevnetManifest      string        `long:"devnet-manifest" description:"Path to the generated Devnet manifest file" default:"./devnet-data/manifest.json"`
	DevnetScript        string        `long:"devnet-script" description:"Path to the Devnet control script" default:"./scripts/devnet-up.sh"`
	DevnetNodes         int           `long:"devnet-nodes" description:"Expected Devnet node count when the manifest is missing" default:"3"`
	DevnetActionTimeout time.Duration `long:"devnet-action-timeout" description:"Timeout for local Devnet control actions" default:"2m"`
	NoTLS               bool          `long:"notls" description:"Disable TLS for upstream btcd RPC calls"`
	ObtcMainNet         bool          `long:"obtcmainnet" description:"Connect to the OBTC main network"`
	ObtcTestNet         bool          `long:"obtctestnet" description:"Connect to the OBTC test network"`
	ObtcRegTest         bool          `long:"obtcregtest" description:"Connect to the OBTC regression test network"`
	RPCCert             string        `short:"c" long:"rpccert" description:"RPC server certificate chain for validation"`
	RPCPassword         string        `short:"P" long:"rpcpass" default-mask:"-" description:"RPC password"`
	RPCServer           string        `short:"s" long:"rpcserver" description:"RPC server to connect to" default:"127.0.0.1"`
	RPCUser             string        `short:"u" long:"rpcuser" description:"RPC username"`
	ShowVersion         bool          `short:"V" long:"version" description:"Display version information and exit"`
	TLSSkipVerify       bool          `long:"skipverify" description:"Do not verify TLS certificates (not recommended)"`
	NetworkName         string
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
	case &chaincfg.ObtcMainNetParams:
		return "9528"
	case &chaincfg.ObtcTestNetParams:
		return "19528"
	case &chaincfg.ObtcRegTestParams:
		return "29528"
	default:
		return "9528"
	}
}

func networkName(params *chaincfg.Params) string {
	switch params {
	case &chaincfg.ObtcTestNetParams:
		return "obtctestnet"
	case &chaincfg.ObtcRegTestParams:
		return "obtcregtest"
	default:
		return "obtcmainnet"
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
	if cfg.DevnetActionTimeout <= 0 {
		return nil, fmt.Errorf("--devnet-action-timeout must be positive")
	}
	if cfg.LabActionTimeout <= 0 {
		return nil, fmt.Errorf("--lab-action-timeout must be positive")
	}
	if cfg.LabLogTailLines <= 0 {
		return nil, fmt.Errorf("--lab-log-tail-lines must be positive")
	}
	if cfg.DevnetNodes < 2 || cfg.DevnetNodes > 5 {
		return nil, fmt.Errorf("--devnet-nodes must be between 2 and 5")
	}
	if cfg.Devnet && cfg.TestnetLab {
		return nil, fmt.Errorf("--devnet and --testnet-lab cannot be used together")
	}

	if cfg.Devnet && !cfg.ObtcMainNet && !cfg.ObtcTestNet && !cfg.ObtcRegTest {
		cfg.ObtcRegTest = true
	}
	if cfg.TestnetLab && !cfg.ObtcMainNet && !cfg.ObtcRegTest {
		cfg.ObtcTestNet = true
	}
	if cfg.Devnet {
		cfg.NoTLS = true
		if cfg.RPCUser == "" {
			cfg.RPCUser = "obtc"
		}
		if cfg.RPCPassword == "" {
			cfg.RPCPassword = "obtcpass"
		}
	}

	params, err := networkParams(cfg)
	if err != nil {
		return nil, err
	}
	cfg.NetworkName = networkName(params)

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
	cfg.DevnetManifest = cleanAndExpandPath(cfg.DevnetManifest)
	cfg.DevnetScript = cleanAndExpandPath(cfg.DevnetScript)
	cfg.LabManifest = cleanAndExpandPath(cfg.LabManifest)
	cfg.LabScript = cleanAndExpandPath(cfg.LabScript)
	return cfg, nil
}
