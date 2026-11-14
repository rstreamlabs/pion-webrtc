<!-- SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly> -->
<!-- SPDX-License-Identifier: MIT -->

# rstream fork of Pion WebRTC

This repository maintains the WebRTC implementation used by the rstream video examples. It derives from [Pion WebRTC](https://github.com/pion/webrtc) and retains the upstream Go module identity and MIT license.

The fork adds media-repair, sender-feedback and transport-lifecycle behavior used by the producer/distributor pipeline. Its [maintenance guide](RSTREAM_FORK.md) identifies the extensions that must survive an upstream update.

Use the [fork guide](RSTREAM_FORK.md) to understand the retained changes and compatibility constraints before selecting a revision. Applications pin reviewed immutable replacements in their own module graph; a pseudo-version’s ancestor tag alone does not describe the reviewed source.

| Task                                | Start here                                                                                                                               |
| ----------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| Integrate rstream in an application | [Go reference SDK](https://github.com/rstreamlabs/rstream-go) or [application examples](https://github.com/rstreamlabs/rstream-examples) |
| Understand this fork                | [Retained behavior and maintenance](RSTREAM_FORK.md)                                                                                     |
| Learn the upstream API              | [Pion webrtc](https://github.com/pion/webrtc)                                                                                            |
| Review source examples              | [Examples](examples)                                                                                                                     |
| Contribute to upstream Pion         | [Contribution guide](https://github.com/pion/webrtc/wiki/Contributing)                                                                   |

For rstream-specific changes, use this repository’s issues and preserve the fork regressions. Report vulnerabilities privately through the [rstream security policy](https://github.com/rstreamlabs/.github/blob/main/SECURITY.md). Upstream attribution and license terms are in [LICENSE](LICENSE).
