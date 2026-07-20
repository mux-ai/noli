---
type: Architecture Decision
title: Local CLI Instead of a Service
description: Coding agents access project knowledge through one local binary rather than MCP, HTTP, or a background daemon.
tags:
  - architecture
  - cli
  - local-first
---

## Decision

Keep the agent runtime local and process-scoped: no MCP server, no network transport, no vector database, and no background process. Thin integrations spawn the fixed `noli` executable with argument arrays and consume the frozen JSON protocol. The `okf` executable is retained only as a deprecated compatibility alias.

## Consequences

Installation and debugging stay simple, all agents receive the same deterministic behavior, and repository containment can be enforced without opening a service boundary.

## Relationships

- Applies to: [Noli CLI](/components/noli-cli.md)
- Applies to: [Claude Code Integration](/integrations/claude-code.md)
- Applies to: [Codex Integration](/integrations/codex.md)
- Applies to: [Pi Coding Agent Integration](/integrations/pi-coding-agent.md)
