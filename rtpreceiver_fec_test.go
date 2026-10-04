// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import (
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/flexfec"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

const (
	testFECPrimaryPayloadType = PayloadType(96)
	testFECRTXPayloadType     = PayloadType(97)
	testFECPayloadType        = PayloadType(118)
)

type fecTestRemote struct {
	track    *TrackRemote
	receiver *RTPReceiver
}

func TestRTPReceiverReadsNegotiatedFlexFECAndUnblocksOnStop(t *testing.T) {
	receiverEngine := &MediaEngine{}
	require.NoError(t, registerFECTestCodecs(receiverEngine, true))
	receiverRegistry := &interceptor.Registry{}
	require.NoError(t, ConfigureRTCPReports(receiverRegistry))
	receiverAPI := NewAPI(WithMediaEngine(receiverEngine), WithInterceptorRegistry(receiverRegistry))
	receiver, err := receiverAPI.NewPeerConnection(Configuration{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, receiver.Close()) })
	remote := make(chan fecTestRemote, 1)
	receiver.OnTrack(func(track *TrackRemote, rtpReceiver *RTPReceiver) {
		remote <- fecTestRemote{track: track, receiver: rtpReceiver}
	})
	_, err = receiver.AddTransceiverFromKind(
		RTPCodecTypeVideo,
		RTPTransceiverInit{Direction: RTPTransceiverDirectionRecvonly},
	)
	require.NoError(t, err)
	senderEngine := &MediaEngine{}
	require.NoError(t, registerFECTestCodecs(senderEngine, false))
	senderRegistry := &interceptor.Registry{}
	require.NoError(t, ConfigureFlexFEC03(
		testFECPayloadType,
		senderEngine,
		senderRegistry,
		flexfec.NumMediaPackets(5),
		flexfec.NumFECPackets(1),
	))
	require.NoError(t, ConfigureRTCPReports(senderRegistry))
	senderAPI := NewAPI(WithMediaEngine(senderEngine), WithInterceptorRegistry(senderRegistry))
	sender, err := senderAPI.NewPeerConnection(Configuration{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sender.Close()) })
	track, err := NewTrackLocalStaticRTP(fecTestH264Capability(), "video", "source")
	require.NoError(t, err)
	_, err = sender.AddTrack(track)
	require.NoError(t, err)
	require.NoError(t, signalPair(receiver, sender))
	sequence := uint16(1)
	incoming := waitForFECTestRemote(t, track, remote, &sequence)
	for range 10 {
		require.NoError(t, track.WriteRTP(fecTestPacket(sequence)))
		sequence++
	}
	require.NotZero(t, incoming.track.FecSSRC())
	buffer := make([]byte, receiveMTU)
	size, _, err := incoming.receiver.ReadFEC(buffer)
	require.NoError(t, err)
	var packet rtp.Packet
	require.NoError(t, packet.Unmarshal(buffer[:size]))
	require.Equal(t, uint32(incoming.track.FecSSRC()), packet.SSRC)
	blocked := waitForBlockedFECRead(t, incoming.receiver, buffer)
	require.NoError(t, incoming.receiver.Stop())
	select {
	case readErr := <-blocked:
		require.Error(t, readErr)
	case <-time.After(time.Second):
		require.FailNow(t, "FlexFEC read remained blocked after receiver stop")
	}
}

func waitForFECTestRemote(
	t *testing.T,
	track *TrackLocalStaticRTP,
	remote <-chan fecTestRemote,
	sequence *uint16,
) fecTestRemote {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer deadline.Stop()
	defer ticker.Stop()
	for {
		require.NoError(t, track.WriteRTP(fecTestPacket(*sequence)))
		*sequence++
		select {
		case incoming := <-remote:
			return incoming
		case <-ticker.C:
		case <-deadline.C:
			require.FailNow(t, "primary track was not received")
		}
	}
}

func waitForBlockedFECRead(t *testing.T, receiver *RTPReceiver, buffer []byte) <-chan error {
	t.Helper()
	var blocked chan error
	for range 10 {
		candidate := make(chan error, 1)
		go func() {
			_, _, readErr := receiver.ReadFEC(buffer)
			candidate <- readErr
		}()
		select {
		case readErr := <-candidate:
			require.NoError(t, readErr)
		case <-time.After(25 * time.Millisecond):
			blocked = candidate
		}
		if blocked != nil {
			break
		}
	}
	require.NotNil(t, blocked, "FlexFEC stream never drained")

	return blocked
}

func registerFECTestCodecs(engine *MediaEngine, receiver bool) error {
	primary := RTPCodecParameters{
		RTPCodecCapability: fecTestH264Capability(),
		PayloadType:        testFECPrimaryPayloadType,
	}
	if err := engine.RegisterCodec(primary, RTPCodecTypeVideo); err != nil {
		return err
	}
	rtx := RTPCodecParameters{
		RTPCodecCapability: RTPCodecCapability{
			MimeType:    MimeTypeRTX,
			ClockRate:   90000,
			SDPFmtpLine: fmt.Sprintf("apt=%d", testFECPrimaryPayloadType),
		},
		PayloadType: testFECRTXPayloadType,
	}
	if err := engine.RegisterCodec(rtx, RTPCodecTypeVideo); err != nil {
		return err
	}
	if receiver {
		fec := RTPCodecParameters{
			RTPCodecCapability: RTPCodecCapability{
				MimeType:    MimeTypeFlexFEC03,
				ClockRate:   90000,
				SDPFmtpLine: "repair-window=10000000",
			},
			PayloadType: testFECPayloadType,
		}

		return engine.RegisterCodec(fec, RTPCodecTypeVideo)
	}

	return nil
}

func fecTestH264Capability() RTPCodecCapability {
	return RTPCodecCapability{
		MimeType:    MimeTypeH264,
		ClockRate:   90000,
		SDPFmtpLine: "packetization-mode=1;profile-level-id=42e01f;level-asymmetry-allowed=1",
	}
}

func fecTestPacket(sequence uint16) *rtp.Packet {
	payload := []byte{0x65, 0, 0}
	binary.BigEndian.PutUint16(payload[1:], sequence)

	return &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    uint8(testFECPrimaryPayloadType),
			SequenceNumber: sequence,
			Timestamp:      uint32(sequence) * 3000,
		},
		Payload: payload,
	}
}
