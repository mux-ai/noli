---
type: Workflow
title: Prepare Agent Context
description: Materialize named retrieval queries into reproducible context files before an agent session.
tags:
  - agents
  - context
  - manifest
---

## Steps

1. Define bounded named queries in `noli-agent-queries.yaml`.
2. Run `noli prepare-agent-context` outside the knowledge root.
3. Verify `manifest.json` bundle ID and file checksums.
4. Point an agent at the named Markdown context.
5. Rebuild when the source bundle ID changes.

## Relationships

- Enforced by: [Single Writer Per Target](/rules/single-writer-per-target.md)
- Uses: [Bounded Retrieval Engine](/components/bounded-retrieval-engine.md)
- Uses: [Prepared Agent Context](/concepts/prepared-agent-context.md)
