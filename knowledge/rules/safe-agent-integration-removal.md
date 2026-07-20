---
type: Business Rule
title: Safe Agent Integration Removal
description: User-global uninstall removes only Noli-managed agent assets while preserving project knowledge and unrelated or modified user files.
tags:
  - agents
  - global
  - lifecycle
  - safety
  - uninstall
---

## Statement

Global uninstallers remove the marked Noli guidance block and only delete skill, command, and extension files that still match their shipped sources. Symlinks, modified files, malformed marker blocks, repository-local integrations, project knowledge, opt-out state, and the CLI are preserved. Codex and Pi retain their shared skill until neither integration remains, and repeated uninstall is safe.

## Relationships

- Applies to: [Claude Code Integration](/integrations/claude-code.md)
- Applies to: [Codex Integration](/integrations/codex.md)
- Applies to: [Pi Coding Agent Integration](/integrations/pi-coding-agent.md)
- Enforced by: [Run Acceptance Gates](/workflows/run-acceptance-gates.md)
- Follows: [Agent Global First-Run Choice](/rules/agent-global-first-run-choice.md)
