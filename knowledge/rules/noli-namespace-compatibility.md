---
type: Business Rule
title: Noli Namespace Compatibility
description: Noli owns the primary product namespace while legacy OKF product identifiers remain bounded deprecated aliases.
tags:
  - compatibility
  - migration
  - namespace
  - noli
---

## Statement

New projects and documentation use `noli`, `noli.yaml`, `.noli/`, `noli-agent-queries.yaml`, `NOLI_BINARY_PATH`, `/noli-context`, and the five `noli_*` Pi tool IDs. During migration, the executable, configuration loader, disabled sentinel, environment resolver, and agent integrations may accept their former `okf` names only as deprecated fallbacks. When both names exist, the Noli name wins. OKF remains the correct name for the Open Knowledge Format standard, its bundles, conformance rules, and parser/SDK types.

## Relationships

- Applies to: [Noli CLI](/components/noli-cli.md)
- Applies to: [Noli](/concepts/noli.md)
- Applies to: [Claude Code Integration](/integrations/claude-code.md)
- Applies to: [Codex Integration](/integrations/codex.md)
- Applies to: [Pi Coding Agent Integration](/integrations/pi-coding-agent.md)
- Enforced by: [Run Acceptance Gates](/workflows/run-acceptance-gates.md)
