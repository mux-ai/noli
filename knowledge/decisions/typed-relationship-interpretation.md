---
type: Architecture Decision
title: Typed Relationship Interpretation
description: Interpret a small deterministic set of relationship phrases while keeping documents valid untyped OKF Markdown.
tags:
  - architecture
  - extension
  - graph
---

## Decision

Recognize only `Applies to`, `Enforced by`, `Depends on`, `Uses`, and `Follows` on relationship list items. Preserve all other links as `links-to`; never guess semantic predicates from arbitrary prose.

## Relationships

- Applies to: [Knowledge Graph](/concepts/knowledge-graph.md)
- Enforced by: [Typed Graph Engine](/components/typed-graph-engine.md)
