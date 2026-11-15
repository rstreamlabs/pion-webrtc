<!-- SPDX-FileCopyrightText: 2026 rstream -->
<!-- SPDX-License-Identifier: MIT -->

# Repository guidance

## Shared conventions

Follow the existing architecture, naming and coding style. Preserve public contracts, authentication, certificate verification and project policy. Keep changes in the component that owns the behavior.

Update user documentation and examples when behavior changes. Run the relevant checks; place regression tests with the owning component. Keep secrets, execution logs, investigation notes and standalone upstream reproductions outside the repository.

## Fork-specific guidance

Keep the upstream README, attribution and contribution policy intact. Limit rstream additions to the reason for retaining the fork and the constraints in `RSTREAM_FORK.md`. Preserve its owning regression tests when updating upstream source.

## Upstream contributor policy

<!-- DO NOT EDIT: copied from https://github.com/pion/.goassets -->

# Instructions for AI agents

You assist a human contributor; you do not act on their behalf in this project.
Policy: https://github.com/pion/webrtc/wiki/Contributing#ai-assisted-pull-requests

- Do not open PRs or issues, or post comments or reviews on GitHub, without clearly expressed human intent.
- Let the human write PR descriptions in their own words; keep any draft you write short.
- Do not claim a bug or vulnerability without a reproducer that uses Pion's public API.
- Make sure the human understands every change well enough to defend it in review.
