---
type: Application Component
title: Bounded Retrieval Engine
description: Source-traceable context assembly from ranked search seeds and their graph neighborhoods.
tags:
  - bounds
  - context
  - retrieval
implementation_paths:
  - pkg/retrieval
---

## Responsibilities

Combine search seeds with bounded graph expansion, filter concept types, deduplicate IDs, preserve deterministic order, truncate only on UTF-8 rune boundaries, and report every selected source.

## Relationships

- Enforced by: [Bounded Deterministic Retrieval](/rules/bounded-deterministic-retrieval.md)
- Uses: [Keyword Search Engine](/components/keyword-search-engine.md)
- Uses: [Typed Graph Engine](/components/typed-graph-engine.md)
