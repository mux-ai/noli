---
type: Architecture Decision
title: Native Pi Tool Extension
description: Expose OKF read operations as native Pi tools through the installed Pi extension API.
tags:
  - architecture
  - integration
  - pi
---

## Decision

Register exactly five read-only tools with Pi's default extension factory. Resolve the executable and repository from host context, pass cancellation to the child process, and validate the adapter against the actual installed Pi loader.

## Relationships

- Applies to: [Pi Coding Agent Integration](/integrations/pi-coding-agent.md)
- Follows: [Local CLI Instead of a Service](/decisions/local-cli-over-service.md)
