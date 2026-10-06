// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/transport/v4"
	"github.com/pion/transport/v4/stdnet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type closeBlockingNet struct {
	transport.Net
	blocked atomic.Bool
	entered chan struct{}
	mu      sync.Mutex
	sockets []*closeBlockingUDPConn
}

type closeBlockingUDPConn struct {
	transport.UDPConn
	owner  *closeBlockingNet
	closed chan struct{}
	once   sync.Once
}

func (n *closeBlockingNet) ListenUDP(network string, address *net.UDPAddr) (transport.UDPConn, error) {
	conn, err := n.Net.ListenUDP(network, address)
	if err != nil {
		return nil, err
	}
	wrapped := &closeBlockingUDPConn{UDPConn: conn, owner: n, closed: make(chan struct{})}
	n.mu.Lock()
	n.sockets = append(n.sockets, wrapped)
	n.mu.Unlock()

	return wrapped, nil
}

func (n *closeBlockingNet) closeSockets() {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, conn := range n.sockets {
		_ = conn.Close()
	}
}

func (c *closeBlockingUDPConn) WriteTo(packet []byte, address net.Addr) (int, error) {
	if c.owner.blocked.Load() {
		select {
		case c.owner.entered <- struct{}{}:
		default:
		}
		<-c.closed

		return 0, io.ErrClosedPipe
	}

	return c.UDPConn.WriteTo(packet, address)
}

func (c *closeBlockingUDPConn) Close() error {
	var err error
	c.once.Do(func() {
		close(c.closed)
		err = c.UDPConn.Close()
	})

	return err
}

func TestPeerConnectionCloseInterruptsBlockedTransportWrite(t *testing.T) { //nolint:cyclop
	for _, graceful := range []bool{false, true} {
		name := "Close"
		if graceful {
			name = "GracefulClose"
		}
		t.Run(name, func(t *testing.T) {
			nativeNet, err := stdnet.NewNet()
			require.NoError(t, err)
			network := &closeBlockingNet{Net: nativeNet, entered: make(chan struct{}, 1)}
			defer network.closeSockets()
			settings := SettingEngine{}
			settings.SetNet(network)
			settings.SetNetworkTypes([]NetworkType{NetworkTypeUDP4})
			settings.SetIncludeLoopbackCandidate(true)
			settings.SetIPFilter(func(ip net.IP) bool { return ip.IsLoopback() })
			offer, err := NewAPI(WithSettingEngine(settings)).NewPeerConnection(Configuration{})
			require.NoError(t, err)
			defer func() {
				network.closeSockets()
				_ = offer.Close()
			}()
			settings.SetNet(nativeNet)
			answer, err := NewAPI(WithSettingEngine(settings)).NewPeerConnection(Configuration{})
			require.NoError(t, err)
			defer func() { _ = answer.Close() }()
			_, err = offer.AddTransceiverFromKind(RTPCodecTypeVideo)
			require.NoError(t, err)
			connected := make(chan struct{}, 2)
			for _, peer := range []*PeerConnection{offer, answer} {
				var once sync.Once
				peer.OnConnectionStateChange(func(state PeerConnectionState) {
					if state == PeerConnectionStateConnected {
						once.Do(func() { connected <- struct{}{} })
					}
				})
			}
			require.NoError(t, signalPairWithOptions(offer, answer, withDisableInitialDataChannel(true)))
			for range 2 {
				select {
				case <-connected:
				case <-time.After(5 * time.Second):
					require.FailNow(t, "local peer pair did not connect")
				}
			}
			network.blocked.Store(true)
			finished := make(chan error, 1)
			go func() {
				if graceful {
					finished <- offer.GracefulClose()
				} else {
					finished <- offer.Close()
				}
			}()
			select {
			case <-network.entered:
			case <-time.After(time.Second):
				assert.Fail(t, "close did not exercise a blocked transport write")
			}
			select {
			case closeErr := <-finished:
				require.NoError(t, closeErr)
			case <-time.After(3 * time.Second):
				// Release the fixture even on the unfixed implementation, so the
				// regression reports a failure instead of leaking its close worker.
				network.closeSockets()
				select {
				case <-finished:
				case <-time.After(time.Second):
					require.FailNow(t, "close worker did not exit after releasing the socket")
				}
				assert.Fail(t, "peer close remained blocked on transport I/O")
			}
			require.Equal(t, PeerConnectionStateClosed, offer.ConnectionState())
		})
	}
}
