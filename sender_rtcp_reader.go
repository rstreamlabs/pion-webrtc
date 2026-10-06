// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"errors"
	"io"
	"slices"
	"sync"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/srtp/v3"
	"github.com/pion/transport/v4/packetio"
)

type rtcpReadCloser interface {
	io.ReadCloser
	SetReadDeadline(time.Time) error
}

// senderRTCPReader merges feedback addressed to an encoding's primary, RTX and
// FEC SSRCs. TWCC feedback describes the whole transport even when its MediaSSRC
// identifies a repair stream. All such feedback must reach the sender interceptor.
// The buffer has the same byte limit as a default SRTCP stream; writes never block
// the transport. Encodings without repair SSRCs retain the direct reader path.
type senderRTCPReader struct {
	streams  []*srtp.ReadStreamSRTCP
	buffer   *packetio.Buffer
	workers  sync.WaitGroup
	stopOnce sync.Once
	readErr  error // Written before buffer.Close; read after Read returns EOF.
	closeErr error // Written by stopOnce, read after stopOnce.Do returns.
}

func openSenderRTCPReader(session *srtp.SessionSRTCP, primary, rtx, fec SSRC) (rtcpReadCloser, error) {
	ssrcs := []SSRC{primary}
	for _, ssrc := range []SSRC{rtx, fec} {
		if ssrc != 0 && !slices.Contains(ssrcs, ssrc) {
			ssrcs = append(ssrcs, ssrc)
		}
	}
	streams := make([]*srtp.ReadStreamSRTCP, 0, len(ssrcs))
	for _, ssrc := range ssrcs {
		stream, err := session.OpenReadStream(uint32(ssrc))
		if err != nil {
			for _, opened := range streams {
				err = errors.Join(err, opened.Close())
			}

			return nil, err
		}
		streams = append(streams, stream)
	}
	if len(streams) == 1 {
		return streams[0], nil
	}
	reader := &senderRTCPReader{streams: streams, buffer: packetio.NewBuffer()}
	reader.buffer.SetLimitSize(100 * 1000)
	reader.workers.Add(len(streams))
	for index, stream := range streams {
		go reader.forward(stream, ssrcs[:index])
	}

	return reader, nil
}

func (r *senderRTCPReader) forward(stream *srtp.ReadStreamSRTCP, earlier []SSRC) {
	defer r.workers.Done()
	// One reusable buffer per SSRC; accommodate the largest packet packetio can
	// deliver, independently of the application's chosen Read buffer size.
	packet := make([]byte, 65535)
	for {
		n, err := stream.Read(packet)
		if err != nil {
			r.stop(err)

			return
		}
		if !forwardSenderRTCP(packet[:n], earlier) {
			continue
		}
		if _, err = r.buffer.Write(packet[:n]); err != nil && !errors.Is(err, packetio.ErrFull) {
			r.stop(err)

			return
		}
	}
}

// SRTCP dispatches an individual packet to each of its destinations. Reports
// covering several SSRCs of this encoding must only be delivered once. The first
// matching reader owns them, independent of goroutine scheduling. Packets without
// destination information remain visible, including on a repair-only compound.
func forwardSenderRTCP(raw []byte, earlier []SSRC) bool {
	if len(earlier) == 0 {
		return true
	}
	packets, err := rtcp.Unmarshal(raw)
	if err != nil || len(packets) != 1 {
		return true
	}
	destinations := packets[0].DestinationSSRC()
	if len(destinations) == 0 {
		return true
	}
	for _, destination := range destinations {
		for _, ssrc := range earlier {
			if destination == uint32(ssrc) {
				return false
			}
		}
	}

	return true
}

func (r *senderRTCPReader) stop(err error) {
	r.stopOnce.Do(func() {
		r.readErr = err
		for _, stream := range r.streams {
			r.closeErr = errors.Join(r.closeErr, stream.Close())
		}
		r.closeErr = errors.Join(r.closeErr, r.buffer.Close())
	})
}

func (r *senderRTCPReader) Read(packet []byte) (int, error) {
	n, err := r.buffer.Read(packet)
	if errors.Is(err, io.EOF) {
		return n, r.readErr
	}

	return n, err
}

func (r *senderRTCPReader) SetReadDeadline(deadline time.Time) error {
	return r.buffer.SetReadDeadline(deadline)
}

func (r *senderRTCPReader) Close() error {
	r.stop(io.EOF)
	r.workers.Wait()

	return r.closeErr
}
