---
type: Business Rule
title: Agent Global First-Run Choice
description: User-global agent guidance asks once before enabling Noli in a repository that has no recorded Noli state.
tags:
  - agents
  - bootstrap
  - claude
  - codex
  - consent
  - global
  - pi
---

## Statement

Each agent's global installer places the shared skill, guidance, and any native command or extension in that agent's documented user scope. A repository with `noli.yaml` or `knowledge/` is enabled without a question; `.noli/disabled` records an opt-out. When neither state exists, Claude Code, Codex, and Pi ask once whether to initialize Noli before implementation. Existing global instructions must be preserved, Codex and Pi reuse `$HOME/.agents/skills`, and repeated installation must not duplicate managed guidance.

## Relationships

- Applies to: [Claude Code Integration](/integrations/claude-code.md)
- Applies to: [Codex Integration](/integrations/codex.md)
- Applies to: [Pi Coding Agent Integration](/integrations/pi-coding-agent.md)
- Enforced by: [Run Acceptance Gates](/workflows/run-acceptance-gates.md)
- Follows: [Bootstrap Repository Knowledge](/workflows/bootstrap-repository-knowledge.md)
