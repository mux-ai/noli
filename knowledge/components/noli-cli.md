---
type: Application Component
title: Noli CLI
description: The local noli executable that powers Noli for agents and developers, with okf retained as a deprecated alias.
tags:
  - cli
  - json
  - local-tools
commands:
  - status
  - list
  - search
  - retrieve
  - get
  - graph
  - validate
  - drift
  - generate
  - prepare-agent-context
implementation_paths:
  - cmd/noli
  - cmd/okf
  - internal/cli
---

## Responsibilities

Parse bounded command arguments, call the public packages, emit one versioned JSON envelope on stdout, keep successful stderr empty, and map failures to stable error codes and documented exit statuses.

## Relationships

- Enforced by: [Frozen Protocol Compatibility](/rules/frozen-protocol-compatibility.md)
- Follows: [Local CLI Instead of a Service](/decisions/local-cli-over-service.md)
- Uses: [Frozen JSON Protocol](/components/json-protocol.md)
- Uses: [Public Go SDK](/components/public-go-sdk.md)
