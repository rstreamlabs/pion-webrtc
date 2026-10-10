// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"testing"

	"github.com/pion/ice/v4"
	stunv3 "github.com/pion/stun/v3"
	"github.com/pion/stun/v4"
	"github.com/stretchr/testify/require"
)

func TestLegacyBindingRequestHandler(t *testing.T) {
	m := stun.MustBuild(stun.TransactionID, stun.BindingRequest, stun.Username("before"))
	se := SettingEngine{}
	pair := &ice.CandidatePair{}
	se.SetICEBindingRequestHandler(func(legacy *stunv3.Message, local, remote ice.Candidate, got *ice.CandidatePair) bool {
		require.Nil(t, local)
		require.Nil(t, remote)
		require.Same(t, pair, got)
		var username stunv3.Username
		require.NoError(t, username.GetFrom(legacy))
		require.Equal(t, "before", username.String())
		require.NoError(t, legacy.Build(stunv3.TransactionID, stunv3.BindingSuccess, stunv3.Username("after")))

		return true
	})
	require.True(t, se.iceBindingRequestHandler(m, nil, nil, pair))
	require.Equal(t, stun.BindingSuccess, m.Type)
	var username stun.Username
	require.NoError(t, username.GetFrom(m))
	require.Equal(t, "after", username.String())
	decoded := &stun.Message{Raw: m.Raw}
	require.NoError(t, decoded.Decode())
	require.Equal(t, m.Type, decoded.Type)
	require.Equal(t, m.TransactionID, decoded.TransactionID)
	require.Equal(t, m.Attributes, decoded.Attributes)
	se.SetICEBindingRequestHandler(nil)
	require.Nil(t, se.iceBindingRequestHandler)
}
