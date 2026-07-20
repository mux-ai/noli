---
type: Agent Integration
title: Pi Coding Agent Integration
description: A project-local or user-global TypeScript extension, shared skill, and guidance exposing five bounded native Noli read tools to Pi.
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

Run `integrations/pi/install.sh` for a target repository, or pass `--global` to install the extension, shared skill, and managed first-run guidance for the current user across repositories. Ensure `noli` is on PATH or set an absolute `NOLI_BINARY_PATH`. The extension rejects shell execution, path escapes, invalid protocol output, timeout, cancellation, and output overflow.

## Relationships

- Enforced by: [Agent Global First-Run Choice](/rules/agent-global-first-run-choice.md)
- Enforced by: [Repository Path Containment](/rules/repository-path-containment.md)
- Follows: [Native Pi Tool Extension](/decisions/native-pi-extension.md)
- Uses: [Frozen JSON Protocol](/components/json-protocol.md)
- Uses: [Noli CLI](/components/noli-cli.md)
