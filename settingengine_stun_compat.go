// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js

package webrtc

import (
	"github.com/pion/ice/v4"
	stunv3 "github.com/pion/stun/v3"
	"github.com/pion/stun/v4"
)

// SetICEBindingRequestHandler installs a STUN v3 binding request callback.
// Candidate arguments and message mutations retain their existing behavior.
//
// Deprecated: Use SetICEBindingRequestHandlerV4 for a native STUN v4 callback.
func (e *SettingEngine) SetICEBindingRequestHandler(
	handler func(m *stunv3.Message, local, remote ice.Candidate, pair *ice.CandidatePair) bool,
) {
	if handler == nil {
		e.iceBindingRequestHandler = nil

		return
	}
	e.iceBindingRequestHandler = func(m *stun.Message, local, remote ice.Candidate, pair *ice.CandidatePair) bool {
		legacy := legacyBindingRequest(m)
		result := handler(legacy, local, remote, pair)
		copyLegacyBindingRequest(m, legacy)

		return result
	}
}

func legacyBindingRequest(m *stun.Message) *stunv3.Message {
	legacy := &stunv3.Message{
		Type:   stunv3.NewType(stunv3.Method(m.Type.Method), stunv3.MessageClass(m.Type.Class)),
		Length: m.Length, TransactionID: m.TransactionID, Raw: m.Raw,
		Attributes: make(stunv3.Attributes, len(m.Attributes)),
	}
	for i, attribute := range m.Attributes {
		legacy.Attributes[i] = stunv3.RawAttribute{
			Type: stunv3.AttrType(attribute.Type), Length: attribute.Length, Value: attribute.Value,
		}
	}

	return legacy
}

func copyLegacyBindingRequest(m *stun.Message, legacy *stunv3.Message) {
	m.Type = stun.NewType(stun.Method(legacy.Type.Method), stun.MessageClass(legacy.Type.Class))
	m.Length, m.TransactionID, m.Raw = legacy.Length, legacy.TransactionID, legacy.Raw
	m.Attributes = m.Attributes[:0]
	for _, attribute := range legacy.Attributes {
		m.Attributes = append(m.Attributes, stun.RawAttribute{
			Type: stun.AttrType(attribute.Type), Length: attribute.Length, Value: attribute.Value,
		})
	}
}
