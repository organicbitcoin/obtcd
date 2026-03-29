package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	var handler http.Handler
	if cfg.Devnet {
		server, err := newDevnetServer(cfg)
		if err != nil {
			log.Fatal(err)
		}
		handler = server.routes()

		log.Printf("Starting obtc-status Devnet dashboard on http://%s", cfg.Listen)
		log.Printf("Devnet manifest: %s", cfg.DevnetManifest)
		log.Printf("Devnet script: %s", cfg.DevnetScript)
		log.Printf("JSON endpoint: http://%s/status", cfg.Listen)
		log.Printf("Health endpoint: http://%s/healthz", cfg.Listen)
	} else {
		rpcCaller, err := newJSONRPCCaller(cfg)
		if err != nil {
			log.Fatal(err)
		}

		server := &statusServer{
			collector: &statusCollector{
				rpc:       rpcCaller,
				rpcServer: cfg.RPCServer,
			},
			refresh: cfg.Refresh,
			timeout: cfg.RPCTimeout,
		}
		handler = server.routes()

		log.Printf("Starting obtc-status on http://%s", cfg.Listen)
		log.Printf("Reading node status from %s", cfg.RPCServer)
		log.Printf("JSON endpoint: http://%s/status", cfg.Listen)
		log.Printf("Health endpoint: http://%s/healthz", cfg.Listen)
	}

	if err := http.ListenAndServe(cfg.Listen, handler); err != nil {
		log.Fatal(fmt.Errorf("obtc-status listen failed: %w", err))
	}
}
