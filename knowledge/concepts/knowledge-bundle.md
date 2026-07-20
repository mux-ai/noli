---
type: Domain Entity
title: Knowledge Bundle
description: A self-contained directory tree of OKF Markdown documents used as the unit of distribution and retrieval.
tags:
  - bundle
  - markdown
  - okf
spec_version: "0.1"
---

## Definition

The repository bundle lives under `knowledge/`. Concept documents carry YAML frontmatter, while optional `index.md` and `log.md` files provide progressive disclosure and history. The bundle is local, diffable, portable, and consumable without a service.

## Relationships

- Follows: [OKF v0.1 Conformance](/rules/okf-v0-1-conformance.md)
- Uses: [Concept Document](/concepts/concept-document.md)
- Uses: [Knowledge Graph](/concepts/knowledge-graph.md)
