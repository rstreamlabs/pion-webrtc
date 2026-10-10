<!-- SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly> -->
<!-- SPDX-License-Identifier: MIT -->

# rstream compatibility branch

This fork retains the negotiated FlexFEC receiver extension, sender-associated
RTCP delivery and bounded peer transport shutdown used by the rstream video
examples. Consumers pin an immutable Go module pseudo-version.

The transport dependencies and STUN imports follow the upstream WebRTC 4.2.20
migration to ICE 4.4.2, STUN 4.0.0 and TURN 5.1.0. This permits the fork and the
rstream Go SDK to share a module graph. The associated upstream remote-ICE-lite
propagation and specification-length test passwords are included; omitting them
can prevent WHEP or static-credential peers from connecting.

Validate the full Go race suite on Linux, plus the FlexFEC, sender RTCP and peer
shutdown regressions. Validate producer/distributor consumers against the pinned
revision before updating their module locks. An upstream version update must
preserve these fork extensions until each is provided by the upstream release.

`SetICEBindingRequestHandler` retains its STUN v3 callback for existing
consumers; `SetICEBindingRequestHandlerV4` installs a native STUN v4 callback.
The legacy adapter copies public message fields in both directions so callback
mutations and candidate-pair decisions remain visible to the ICE agent.
