// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import "time"

// A closing peer gets this grace period for normal protocol shutdown before
// its ICE sockets are closed to interrupt blocked I/O. This does not change
// write deadlines, buffering or transport behavior while the peer is live.
const peerConnectionCloseTransportTimeout = time.Second

func stopICEOnCloseTimeout(transport *ICETransport) func() error {
	if transport == nil {
		return func() error { return nil }
	}
	done := make(chan error, 1)
	timer := time.AfterFunc(peerConnectionCloseTransportTimeout, func() {
		// Non-graceful Stop releases socket I/O without waiting for application
		// callbacks. GracefulClose still joins its normal workers afterward.
		transport.log.Warnf("Interrupting ICE transport after peer connection close exceeded %s",
			peerConnectionCloseTransportTimeout)
		done <- transport.Stop()
	})

	// Called exactly once by the closing owner, including on early return.
	// Join an already-running callback so no shutdown worker outlives Close.
	return func() error {
		if timer.Stop() {
			return nil
		}

		return <-done
	}
}
