---
type: Agent Integration
title: Pi Coding Agent Integration
description: A project-local TypeScript extension exposing five bounded native Noli read tools to Pi.
tags:
  - agents
  - integration
  - pi
  - typescript
implementation_paths:
  - integrations/pi
tools:
  - noli_status
  - noli_search
  - noli_retrieve
  - noli_get
  - noli_graph
verified_api: "0.78"
---

## Usage

Run `integrations/pi/install.sh` for the target repository, ensure `noli` is on PATH or set an absolute `NOLI_BINARY_PATH`, then start Pi from the repository. The extension rejects shell execution, path escapes, invalid protocol output, timeout, cancellation, and output overflow.

## Relationships

- Enforced by: [Repository Path Containment](/rules/repository-path-containment.md)
- Follows: [Native Pi Tool Extension](/decisions/native-pi-extension.md)
- Uses: [Frozen JSON Protocol](/components/json-protocol.md)
- Uses: [Noli CLI](/components/noli-cli.md)
