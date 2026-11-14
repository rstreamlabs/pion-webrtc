<!-- SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly> -->
<!-- SPDX-License-Identifier: MIT -->

# rstream compatibility branch

This fork retains the negotiated FlexFEC receiver extension, sender-associated
RTCP delivery and bounded peer transport shutdown used by the rstream video
examples. Consumers pin an immutable Go module pseudo-version.

The transport dependencies and STUN imports follow the upstream WebRTC 4.2.26
runtime changes (upstream commit `0e35f8059c5a7cb2002b53f8b2017f8148ee2fc7`),
with ICE 4.4.9, SCTP 1.12.1, STUN 4.0.1 and TURN 5.1.3. This permits the fork and the
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

The Linux CI keeps the full race/benchmark suite and cross-package coverage.
GitHub retains its logs, coverage profile and summary for 90 days. This fork
has no Codecov upload credential; evidence stays with the reviewed workflow
instead of relying on the upstream project's upload service.

## Upstream maintenance qualification

The runtime delta from upstream base `0f364c61c11308fbedc4da2bb868f69504632d7d`
to release 4.2.26 is reconciled on the existing fork history. Consumer pins remain
immutable fork revisions; their pseudo-version ancestry is not a substitute for
the reviewed upstream runtime revision above. Fork release/merge automation and
the landing README are retained rather than replaced with upstream workflows.
The security minima remain DTLS 3.1.15 and x/net 0.56.0, with Go 1.25 source
compatibility. First-party applications use their newer reviewed Go 1.27.4 graph.

Transport v5 requires explicit packet attributes on the private merged RTCP
buffer. The merged reader still bounds payload storage at 100,000 bytes, forwards
repair feedback once, preserves deadlines and unblocks workers on close.
Shutdown/repair/STUN/FlexFEC regressions remain in the full Linux race suite.
The saturation fixture also accepts undeclared SRTCP streams, matching the real
PeerConnection processor; otherwise late in-flight packets can leave the fixture
blocked during session cleanup after a registered reader closes. Its 500-packet
load and byte-limit assertions are retained.

A compatible interceptor fork and producer/distributor qualification are required
before repinning video consumers. This update does not retire the repair,
transport-shutdown or legacy STUN callback extensions.
