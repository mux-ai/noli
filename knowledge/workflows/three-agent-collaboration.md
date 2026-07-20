---
type: Workflow
title: Collaborate with Three Code Agents
description: Let Claude Code, Codex, and Pi share deterministic knowledge without allowing output write collisions.
tags:
  - agents
  - claude
  - codex
  - concurrency
  - pi
---

## Steps

1. Start every agent at the same repository root and knowledge bundle.
2. Let read-only status, search, retrieve, get, and graph calls run concurrently.
3. Assign ownership before agents edit the structured concept source.
4. Allow only the process holding the target lock to generate or prepare a shared output.
5. Retry a losing writer after `GENERATION_FAILED`, then validate and retrieve again.

## Relationships

- Enforced by: [Single Writer Per Target](/rules/single-writer-per-target.md)
- Uses: [Claude Code Integration](/integrations/claude-code.md)
- Uses: [Codex Integration](/integrations/codex.md)
- Uses: [Pi Coding Agent Integration](/integrations/pi-coding-agent.md)
