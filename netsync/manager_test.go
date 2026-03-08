package netsync

import (
	"container/list"
	"errors"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/database"
	_ "github.com/btcsuite/btcd/database/ffldb"
	peerpkg "github.com/btcsuite/btcd/peer"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

func createNetsyncTestChain(t *testing.T, params *chaincfg.Params) *blockchain.BlockChain {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "ffldb")
	db, err := database.Create("ffldb", dbPath, params.Net)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	chain, err := blockchain.New(&blockchain.Config{
		DB:               db,
		UtxoCacheMaxSize: 1 << 20,
		ChainParams:      params,
		TimeSource:       blockchain.NewMedianTime(),
		SigCache:         txscript.NewSigCache(1000),
		HashCache:        txscript.NewHashCache(1000),
	})
	require.NoError(t, err)

	return chain
}

func newTestSyncManager(t *testing.T, params *chaincfg.Params) *SyncManager {
	t.Helper()

	DisableLog()

	return &SyncManager{
		chain:           createNetsyncTestChain(t, params),
		chainParams:     params,
		rejectedTxns:    make(map[chainhash.Hash]struct{}),
		requestedTxns:   make(map[chainhash.Hash]struct{}),
		requestedBlocks: make(map[chainhash.Hash]struct{}),
		peerStates:      make(map[*peerpkg.Peer]*peerSyncState),
		progressLogger:  newBlockProgressLogger("Processed", log),
		headerList:      list.New(),
	}
}

func newCandidatePeerState() *peerSyncState {
	return &peerSyncState{
		syncCandidate:   true,
		requestedTxns:   make(map[chainhash.Hash]struct{}),
		requestedBlocks: make(map[chainhash.Hash]struct{}),
	}
}

func newPeerConfig(params *chaincfg.Params, services wire.ServiceFlag,
	lastBlock int32) *peerpkg.Config {

	return &peerpkg.Config{
		NewestBlock: func() (*chainhash.Hash, int32, error) {
			return params.GenesisHash, lastBlock, nil
		},
		UserAgentName:       "netsync-test",
		UserAgentVersion:    "1.0.0",
		ChainParams:         params,
		Services:            services,
		TrickleInterval:     10 * time.Millisecond,
		AllowSelfConns:      true,
		DisableStallHandler: true,
	}
}

func createConnectedSyncPeer(t *testing.T, params *chaincfg.Params,
	remoteHeight int32, remoteServices wire.ServiceFlag,
	addr string) (*peerpkg.Peer, *peerpkg.Peer) {

	t.Helper()

	localCfg := newPeerConfig(params, wire.SFNodeNetwork|wire.SFNodeWitness, 0)
	remoteCfg := newPeerConfig(params, remoteServices, remoteHeight)

	inbound := peerpkg.NewInboundPeer(remoteCfg)
	outbound, err := peerpkg.NewOutboundPeer(localCfg, addr)
	require.NoError(t, err)

	require.NoError(t, setupPeerConnection(inbound, outbound))
	require.Eventually(t, func() bool {
		return inbound.Connected() &&
			outbound.Connected() &&
			inbound.VersionKnown() &&
			outbound.VersionKnown() &&
			inbound.VerAckReceived() &&
			outbound.VerAckReceived()
	}, 2*time.Second, 10*time.Millisecond)

	t.Cleanup(func() {
		outbound.Disconnect()
		inbound.Disconnect()
		outbound.WaitForDisconnect()
		inbound.WaitForDisconnect()
	})

	return outbound, inbound
}

func setupPeerConnection(in, out *peerpkg.Peer) error {
	listenFunc := func(l *net.TCPListener, errChan chan error,
		listenChan chan struct{}) {

		listenChan <- struct{}{}

		conn, err := l.Accept()
		if err != nil {
			errChan <- err
			return
		}

		in.AssociateConnection(conn)
		errChan <- nil
	}

	dialFunc := func(addr *net.TCPAddr) error {
		conn, err := net.Dial("tcp", addr.String())
		if err != nil {
			return err
		}

		out.AssociateConnection(conn)
		return nil
	}

	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return err
	}

	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	errChan := make(chan error, 1)
	listenChan := make(chan struct{}, 1)

	go listenFunc(listener, errChan, listenChan)
	<-listenChan

	if err := dialFunc(listener.Addr().(*net.TCPAddr)); err != nil {
		return err
	}

	select {
	case err = <-errChan:
		return err
	case <-time.After(2 * time.Second):
		return errors.New("failed to create peer connection")
	}
}

func TestStartSyncSkipsDisconnectedCandidates(t *testing.T) {
	params := &chaincfg.SimNetParams
	sm := newTestSyncManager(t, params)

	fallbackPeer, _ := createConnectedSyncPeer(
		t, params, 0, wire.SFNodeNetwork|wire.SFNodeWitness, "127.0.0.1:18555",
	)
	disconnectedPeer, _ := createConnectedSyncPeer(
		t, params, 7, wire.SFNodeNetwork|wire.SFNodeWitness, "127.0.0.1:18556",
	)
	disconnectedPeer.Disconnect()
	require.Eventually(t, func() bool {
		return !disconnectedPeer.Connected()
	}, time.Second, 10*time.Millisecond)

	sm.peerStates[fallbackPeer] = newCandidatePeerState()
	sm.peerStates[disconnectedPeer] = newCandidatePeerState()

	sm.startSync()

	require.Same(t, fallbackPeer, sm.syncPeer)
}

func TestHandleDonePeerMsgPromotesBackupPeerAndClearsRequestedState(t *testing.T) {
	params := &chaincfg.SimNetParams
	sm := newTestSyncManager(t, params)

	backupPeer, _ := createConnectedSyncPeer(
		t, params, 4, wire.SFNodeNetwork|wire.SFNodeWitness, "127.0.0.1:18557",
	)
	syncPeer, _ := createConnectedSyncPeer(
		t, params, 8, wire.SFNodeNetwork|wire.SFNodeWitness, "127.0.0.1:18558",
	)

	backupState := newCandidatePeerState()
	syncState := newCandidatePeerState()
	sm.peerStates[backupPeer] = backupState
	sm.peerStates[syncPeer] = syncState
	sm.syncPeer = syncPeer

	txHash := chainhash.Hash{0x01}
	blockHash := chainhash.Hash{0x02}
	sm.requestedTxns[txHash] = struct{}{}
	sm.requestedBlocks[blockHash] = struct{}{}
	syncState.requestedTxns[txHash] = struct{}{}
	syncState.requestedBlocks[blockHash] = struct{}{}

	sm.handleDonePeerMsg(syncPeer)

	_, exists := sm.peerStates[syncPeer]
	require.False(t, exists)
	require.Empty(t, sm.requestedTxns)
	require.Empty(t, sm.requestedBlocks)
	require.Same(t, backupPeer, sm.syncPeer)
}

func TestHandleStallSampleSkipsDisconnectedSyncPeerAndFailsOver(t *testing.T) {
	params := &chaincfg.SimNetParams
	sm := newTestSyncManager(t, params)

	backupPeer, _ := createConnectedSyncPeer(
		t, params, 0, wire.SFNodeNetwork|wire.SFNodeWitness, "127.0.0.1:18559",
	)
	stalledPeer, _ := createConnectedSyncPeer(
		t, params, 5, wire.SFNodeNetwork|wire.SFNodeWitness, "127.0.0.1:18560",
	)

	backupState := newCandidatePeerState()
	stalledState := newCandidatePeerState()
	sm.peerStates[backupPeer] = backupState
	sm.peerStates[stalledPeer] = stalledState
	sm.syncPeer = stalledPeer
	sm.lastProgressTime = time.Now().Add(-maxStallDuration - time.Second)

	txHash := chainhash.Hash{0x03}
	blockHash := chainhash.Hash{0x04}
	sm.requestedTxns[txHash] = struct{}{}
	sm.requestedBlocks[blockHash] = struct{}{}
	stalledState.requestedTxns[txHash] = struct{}{}
	stalledState.requestedBlocks[blockHash] = struct{}{}

	sm.handleStallSample()

	require.Eventually(t, func() bool {
		return !stalledPeer.Connected()
	}, time.Second, 10*time.Millisecond)
	require.Empty(t, sm.requestedTxns)
	require.Empty(t, sm.requestedBlocks)
	require.Same(t, backupPeer, sm.syncPeer)
}

func TestHandleInvMsgIgnoresNonSyncPeerWhileCatchingUp(t *testing.T) {
	params := &chaincfg.SimNetParams
	sm := newTestSyncManager(t, params)

	syncPeer, _ := createConnectedSyncPeer(
		t, params, 4, wire.SFNodeNetwork|wire.SFNodeWitness, "127.0.0.1:18561",
	)
	otherPeer, _ := createConnectedSyncPeer(
		t, params, 6, wire.SFNodeNetwork|wire.SFNodeWitness, "127.0.0.1:18562",
	)

	sm.syncPeer = syncPeer
	sm.peerStates[syncPeer] = newCandidatePeerState()
	otherState := newCandidatePeerState()
	sm.peerStates[otherPeer] = otherState

	msg := wire.NewMsgInv()
	msg.AddInvVect(wire.NewInvVect(wire.InvTypeBlock, &chainhash.Hash{0x05}))

	sm.handleInvMsg(&invMsg{
		inv:  msg,
		peer: otherPeer,
	})

	require.Empty(t, otherState.requestQueue)
	require.Empty(t, sm.requestedBlocks)
	require.Empty(t, sm.requestedTxns)
}

func TestHandleBlockMsgDisconnectsPeerForUnrequestedBlock(t *testing.T) {
	params := &chaincfg.SimNetParams
	sm := newTestSyncManager(t, params)

	blockPeer, _ := createConnectedSyncPeer(
		t, params, 2, wire.SFNodeNetwork|wire.SFNodeWitness, "127.0.0.1:18563",
	)
	sm.peerStates[blockPeer] = newCandidatePeerState()

	msgBlock := wire.NewMsgBlock(&wire.BlockHeader{
		Version:    1,
		PrevBlock:  *params.GenesisHash,
		MerkleRoot: chainhash.Hash{0x06},
		Timestamp:  time.Unix(1, 0),
		Bits:       params.PowLimitBits,
		Nonce:      1,
	})
	block := btcutil.NewBlock(msgBlock)

	sm.handleBlockMsg(&blockMsg{
		block: block,
		peer:  blockPeer,
	})

	require.Eventually(t, func() bool {
		return !blockPeer.Connected()
	}, time.Second, 10*time.Millisecond)
}
