---
type: Application Component
title: Frozen JSON Protocol
description: Versioned deterministic success and error envelopes shared by all local agent integrations.
tags:
  - compatibility
  - json
  - protocol
implementation_paths:
  - pkg/protocol
  - docs/PROTOCOL.md
version: 1
---

## Responsibilities

Define explicit response DTOs, non-null deterministic arrays, stdout and stderr behavior, stable error codes, and command exit mappings. Golden fixtures are checksum-locked.

## Relationships

- Applies to: [Noli CLI](/components/noli-cli.md)
- Enforced by: [Frozen Protocol Compatibility](/rules/frozen-protocol-compatibility.md)
