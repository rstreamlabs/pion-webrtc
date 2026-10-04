// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/srtp/v3"
	"github.com/pion/transport/v4/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func senderRTCPTestSession(t *testing.T) (*srtp.SessionSRTCP, *srtp.WriteStreamSRTCP) {
	t.Helper()
	local, remote := net.Pipe()
	config := &srtp.Config{
		Profile: srtp.ProtectionProfileAes128CmHmacSha1_80,
		Keys: srtp.SessionKeys{
			LocalMasterKey: make([]byte, 16), LocalMasterSalt: make([]byte, 14),
			RemoteMasterKey: make([]byte, 16), RemoteMasterSalt: make([]byte, 14),
		},
	}
	reader, err := srtp.NewSessionSRTCP(local, config)
	require.NoError(t, err)
	writer, err := srtp.NewSessionSRTCP(remote, config)
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, reader.Close())
		assert.NoError(t, writer.Close())
	})
	stream, err := writer.OpenWriteStream()
	require.NoError(t, err)
	require.NoError(t, stream.SetWriteDeadline(time.Now().Add(5*time.Second)))

	return reader, stream
}

func TestSenderRTCPReaderRepairFeedbackAndDeadlines(t *testing.T) {
	t.Cleanup(test.CheckRoutines(t))
	session, writer := senderRTCPTestSession(t)
	reader, err := openSenderRTCPReader(session, 1, 2, 3)
	require.NoError(t, err)
	defer func() { assert.NoError(t, reader.Close()) }()
	buffer := make([]byte, 1500)
	require.NoError(t, reader.SetReadDeadline(time.Now().Add(-time.Second)))
	_, err = reader.Read(buffer)
	var timeout net.Error
	require.ErrorAs(t, err, &timeout)
	assert.True(t, timeout.Timeout())
	require.NoError(t, reader.SetReadDeadline(time.Now().Add(time.Second)))
	for _, ssrc := range []uint32{1, 2, 3, 2, 1} {
		raw, marshalErr := (&rtcp.PictureLossIndication{SenderSSRC: 42, MediaSSRC: ssrc}).Marshal()
		require.NoError(t, marshalErr)
		_, writeErr := writer.Write(raw)
		require.NoError(t, writeErr)
		n, readErr := reader.Read(buffer)
		require.NoError(t, readErr)
		assert.Equal(t, raw, buffer[:n])
	}
	// A caller's short buffer consumes only that packet, without killing the
	// forwarding workers or concealing the error from the caller.
	raw, err := (&rtcp.PictureLossIndication{SenderSSRC: 42, MediaSSRC: 3}).Marshal()
	require.NoError(t, err)
	_, err = writer.Write(raw)
	require.NoError(t, err)
	_, err = reader.Read(buffer[:1])
	assert.ErrorIs(t, err, io.ErrShortBuffer)
	_, err = writer.Write(raw)
	require.NoError(t, err)
	_, err = reader.Read(buffer)
	require.NoError(t, err)
}

func TestSenderRTCPReaderDeliversMultiSSRCReportOnce(t *testing.T) {
	t.Cleanup(test.CheckRoutines(t))
	session, writer := senderRTCPTestSession(t)
	reader, err := openSenderRTCPReader(session, 1, 2, 3)
	require.NoError(t, err)
	defer func() { assert.NoError(t, reader.Close()) }()
	for _, destinations := range [][]uint32{{1, 2, 3}, {2, 3}} {
		report := &rtcp.ReceiverReport{SSRC: 42}
		for _, ssrc := range destinations {
			report.Reports = append(report.Reports, rtcp.ReceptionReport{SSRC: ssrc})
		}
		raw, marshalErr := report.Marshal()
		require.NoError(t, marshalErr)
		require.NoError(t, reader.SetReadDeadline(time.Now().Add(time.Second)))
		_, writeErr := writer.Write(raw)
		require.NoError(t, writeErr)
		buffer := make([]byte, 1500)
		n, readErr := reader.Read(buffer)
		require.NoError(t, readErr)
		assert.Equal(t, raw, buffer[:n])
		require.NoError(t, reader.SetReadDeadline(time.Now().Add(20*time.Millisecond)))
		_, readErr = reader.Read(buffer)
		var timeout net.Error
		require.ErrorAs(t, readErr, &timeout)
		assert.True(t, timeout.Timeout())
	}
}

func TestSenderRTCPReaderCloseUnblocksReads(t *testing.T) {
	t.Cleanup(test.CheckRoutines(t))
	session, _ := senderRTCPTestSession(t)
	reader, err := openSenderRTCPReader(session, 1, 2, 3)
	require.NoError(t, err)
	var readers sync.WaitGroup
	for range 8 {
		readers.Add(1)
		go func() {
			defer readers.Done()
			_, readErr := reader.Read(make([]byte, 1500))
			assert.ErrorIs(t, readErr, io.EOF)
		}()
	}
	var closers sync.WaitGroup
	for range 8 {
		closers.Add(1)
		go func() {
			defer closers.Done()
			assert.NoError(t, reader.Close())
		}()
	}
	closers.Wait()
	readers.Wait()
}

func TestSenderRTCPReaderDeduplicatesConfiguredSSRCs(t *testing.T) {
	t.Cleanup(test.CheckRoutines(t))
	session, _ := senderRTCPTestSession(t)
	reader, err := openSenderRTCPReader(session, 1, 1, 0)
	require.NoError(t, err)
	_, direct := reader.(*srtp.ReadStreamSRTCP)
	assert.True(t, direct, "primary-only encoding must not start forwarding goroutines")
	require.NoError(t, reader.Close())
	reader, err = openSenderRTCPReader(session, 4, 5, 5)
	require.NoError(t, err)
	merged, ok := reader.(*senderRTCPReader)
	require.True(t, ok)
	assert.Len(t, merged.streams, 2)
	require.NoError(t, reader.Close())
}

func TestSenderRTCPReaderBoundedWhenApplicationDoesNotRead(t *testing.T) {
	t.Cleanup(test.CheckRoutines(t))
	session, writer := senderRTCPTestSession(t)
	reader, err := openSenderRTCPReader(session, 1, 2, 0)
	require.NoError(t, err)
	merged, ok := reader.(*senderRTCPReader)
	require.True(t, ok)
	raw, err := (&rtcp.ReceiverReport{
		SSRC: 42, Reports: []rtcp.ReceptionReport{{SSRC: 2}},
		ProfileExtensions: make([]byte, 1200),
	}).Marshal()
	require.NoError(t, err)
	for range 500 {
		_, err = writer.Write(raw)
		require.NoError(t, err)
	}
	assert.LessOrEqual(t, merged.buffer.Size(), 100*1000)
	assert.NoError(t, reader.Close())
}

func TestSenderRTCPReaderUnderlyingCloseStopsAllReaders(t *testing.T) {
	t.Cleanup(test.CheckRoutines(t))
	session, _ := senderRTCPTestSession(t)
	reader, err := openSenderRTCPReader(session, 1, 2, 3)
	require.NoError(t, err)
	merged, ok := reader.(*senderRTCPReader)
	require.True(t, ok)
	require.NoError(t, reader.SetReadDeadline(time.Now().Add(time.Second)))
	require.NoError(t, merged.streams[1].Close())
	_, err = reader.Read(make([]byte, 1500))
	assert.ErrorIs(t, err, io.EOF)
	assert.NoError(t, reader.Close())
}
