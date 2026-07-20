---
type: Business Rule
title: Frozen Protocol Compatibility
description: Agent-visible JSON shapes, error codes, and exit mappings change only through an explicit contract update.
tags:
  - compatibility
  - fixtures
  - protocol
---

## Statement

Every command emits protocol version 1 with one JSON line on stdout. Success leaves stderr empty. Golden fixtures and their checksum lock must pass before a protocol change is accepted.

## Relationships

- Applies to: [Frozen JSON Protocol](/components/json-protocol.md)
- Enforced by: [Run Acceptance Gates](/workflows/run-acceptance-gates.md)
