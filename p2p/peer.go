package p2p

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/Sukhavati-Labs/go-miner/config"
	"github.com/Sukhavati-Labs/go-miner/consensus"
	"github.com/Sukhavati-Labs/go-miner/logging"
	"github.com/Sukhavati-Labs/go-miner/p2p/connection"
	gocrypto "github.com/Sukhavati-Labs/skt-tendermint/go-crypto"
	gowire "github.com/Sukhavati-Labs/skt-tendermint/go-wire"
	cmn "github.com/Sukhavati-Labs/skt-tendermint/tmlibs/common"
	"github.com/pkg/errors"
)

// peerConn contains the raw connection and its config.
type peerConn struct {
	outbound bool
	config   *PeerConfig
	conn     net.Conn // source connection
}

// PeerConfig is a Peer configuration.
type PeerConfig struct {
	HandshakeTimeout time.Duration           `mapstructure:"handshake_timeout"` // times are in seconds
	DialTimeout      time.Duration           `mapstructure:"dial_timeout"`
	MConfig          *connection.MConnConfig `mapstructure:"connection"`
}

// DefaultPeerConfig returns the default config.
func DefaultPeerConfig(config *config.Config) *PeerConfig {
	return &PeerConfig{
		HandshakeTimeout: time.Duration(config.Network.P2P.HandshakeTimeout) * time.Second, // * time.Second,
		DialTimeout:      time.Duration(config.Network.P2P.DialTimeout) * time.Second,      // * time.Second,
		MConfig:          connection.DefaultMConnConfig(),
	}
}

// Peer represent a miner network node
type Peer struct {
	cmn.BaseService
	*NodeInfo
	*peerConn
	mconn *connection.MConnection // multiplex connection
	Key   string
}

// OnStart implements BaseService.
func (p *Peer) OnStart() error {
	p.BaseService.OnStart()
	_, err := p.mconn.Start()
	return err
}

// OnStop implements BaseService.
func (p *Peer) OnStop() {
	p.BaseService.OnStop()
	p.mconn.Stop()
}

func newPeer(pc *peerConn, nodeInfo *NodeInfo, reactorsByCh map[byte]Reactor, chDescs []*connection.ChannelDescriptor, onPeerError func(*Peer, interface{})) *Peer {
	// Key and NodeInfo are set after Handshake
	p := &Peer{
		peerConn: pc,
		NodeInfo: nodeInfo,
		Key:      nodeInfo.PubKey.KeyString(),
	}
	p.mconn = createMConnection(pc.conn, p, reactorsByCh, chDescs, onPeerError, pc.config.MConfig)
	p.BaseService = *cmn.NewBaseService(nil, "Peer", p)
	return p
}

func newOutboundPeerConn(addr *NetAddress, ourNodePrivKey gocrypto.PrivKeyEd25519, config *PeerConfig) (*peerConn, error) {
	conn, err := dial(addr, config)
	if err != nil {
		return nil, errors.Wrap(err, "Error dial peer")
	}

	pc, err := newPeerConn(conn, true, ourNodePrivKey, config)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return pc, nil
}

func newInboundPeerConn(conn net.Conn, ourNodePrivKey gocrypto.PrivKeyEd25519, config *config.Config) (*peerConn, error) {
	return newPeerConn(conn, false, ourNodePrivKey, DefaultPeerConfig(config))
}

func newPeerConn(rawConn net.Conn, outbound bool, ourNodePrivKey gocrypto.PrivKeyEd25519, config *PeerConfig) (*peerConn, error) {
	rawConn.SetDeadline(time.Now().Add(config.HandshakeTimeout))
	conn, err := connection.MakeSecretConnection(rawConn, ourNodePrivKey)
	if err != nil {
		return nil, errors.Wrap(err, "Error creating peer")
	}

	return &peerConn{
		config:   config,
		outbound: outbound,
		conn:     conn,
	}, nil
}

// Addr returns peer's remote network address.
func (p *Peer) Addr() net.Addr {
	return p.conn.RemoteAddr()
}

// CanSend returns true if the send queue is not full, false otherwise.
func (p *Peer) CanSend(chID byte) bool {
	if !p.IsRunning() {
		return false
	}
	return p.mconn.CanSend(chID)
}

// CloseConn should be used when the peer was created, but never started.
func (pc *peerConn) CloseConn() {
	pc.conn.Close()
}

// Equals reports whenever 2 peers are actually represent the same node.
func (p *Peer) Equals(other *Peer) bool {
	return p.Key == other.Key
}

// HandshakeTimeout performs a handshake between a given node and the peer.
// NOTE: blocking
func (pc *peerConn) HandshakeTimeout(ourNodeInfo *NodeInfo, timeout time.Duration) (*NodeInfo, error) {
	// Set deadline for handshake so we don't block forever on conn.ReadFull
	if err := pc.conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}

	var peerNodeInfo = new(NodeInfo)
	var err1, err2 error
	cmn.Parallel(
		func() {
			var n int
			gowire.WriteBinary(ourNodeInfo, pc.conn, &n, &err1)
		},
		func() {
			var n int
			gowire.ReadBinary(peerNodeInfo, pc.conn, maxNodeInfoSize, &n, &err2)
			logging.CPrint(logging.INFO, "receive peer handshake", logging.LogFormat{
				"peer_node_info": peerNodeInfo,
				"remoteAddr":     pc.conn.RemoteAddr().String(),
			})
		})
	if err1 != nil {
		return peerNodeInfo, errors.Wrap(err1, "Error during handshake/write")
	}
	if err2 != nil {
		return peerNodeInfo, errors.Wrap(err2, "Error during handshake/read")
	}

	// Remove deadline
	if err := pc.conn.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	peerNodeInfo.RemoteAddr = pc.conn.RemoteAddr().String()
	return peerNodeInfo, nil
}

func (p *Peer) ID() string {
	return p.Key
}

// IsOutbound returns true if the connection is outbound, false otherwise.
func (p *Peer) IsOutbound() bool {
	return p.outbound
}

// PubKey returns peer's public key.
func (p *Peer) PubKey() gocrypto.PubKeyEd25519 {
	return p.conn.(*connection.SecretConnection).RemotePubKey()
}

// Send msg to the channel identified by chID byte. Returns false if the send
// queue is full after timeout, specified by MConnection.
func (p *Peer) Send(chID byte, msg interface{}) bool {
	if !p.IsRunning() {
		return false
	}
	return p.mconn.Send(chID, msg)
}

func (p *Peer) ServiceFlag() consensus.ServiceFlag {
	services := consensus.SFFullNode
	if len(p.Other) == 0 {
		return services
	}

	if serviceFlag, err := strconv.ParseUint(p.Other[0], 10, 64); err == nil {
		services = consensus.ServiceFlag(serviceFlag)
	}
	return services
}

// String representation.
func (p *Peer) String() string {
	if p.outbound {
		return fmt.Sprintf("Peer{%v %v out}", p.mconn, p.Key[:12])
	}
	return fmt.Sprintf("Peer{%v %v in}", p.mconn, p.Key[:12])
}

// TrySend msg to the channel identified by chID byte. Immediately returns
// false if the send queue is full.
func (p *Peer) TrySend(chID byte, msg interface{}) bool {
	if !p.IsRunning() {
		return false
	}
	return p.mconn.TrySend(chID, msg)
}

func createMConnection(conn net.Conn, p *Peer, reactorsByCh map[byte]Reactor, chDescs []*connection.ChannelDescriptor, onPeerError func(*Peer, interface{}), config *connection.MConnConfig) *connection.MConnection {
	onReceive := func(chID byte, msgBytes []byte) {
		reactor := reactorsByCh[chID]
		if reactor == nil {
			logging.CPrint(logging.FATAL, fmt.Sprintf("Unknown channel %X", chID))
		}
		reactor.Receive(chID, p, msgBytes)
	}

	onError := func(r interface{}) {
		onPeerError(p, r)
	}
	return connection.NewMConnectionWithConfig(conn, chDescs, onReceive, onError, config)
}

func dial(addr *NetAddress, config *PeerConfig) (net.Conn, error) {
	conn, err := addr.DialTimeout(config.DialTimeout)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
