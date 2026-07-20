---
type: Business Rule
title: Codex Global First-Run Choice
description: User-global Codex guidance asks once before enabling Noli in a repository that has no recorded Noli state.
tags:
  - agents
  - bootstrap
  - codex
  - consent
  - global
---

## Statement

Global installation places the shared skill under `$HOME/.agents/skills` and adds managed guidance to the active global Codex instruction file. A repository with `noli.yaml` or `knowledge/` is enabled without a question; `.noli/disabled` records an opt-out. When neither state exists, Codex asks once whether to initialize Noli before implementation. Existing global instructions must be preserved and repeated installation must not duplicate the managed guidance block.

## Relationships

- Applies to: [Codex Integration](/integrations/codex.md)
- Enforced by: [Run Acceptance Gates](/workflows/run-acceptance-gates.md)
- Follows: [Bootstrap Repository Knowledge](/workflows/bootstrap-repository-knowledge.md)
