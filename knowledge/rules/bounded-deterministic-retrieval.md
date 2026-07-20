---
type: Business Rule
title: Bounded Deterministic Retrieval
description: Retrieval must be reproducible, size-bounded, UTF-8 safe, and traceable to selected concepts.
tags:
  - determinism
  - retrieval
  - safety
---

## Statement

The same bundle, query, and options must produce byte-identical JSON. Search, graph hops, document count, and context characters are bounded. Context sections are added whole where possible, and any truncation occurs only on rune boundaries with accurate statistics.

## Relationships

- Applies to: [Bounded Retrieval Engine](/components/bounded-retrieval-engine.md)
- Uses: [Knowledge Graph](/concepts/knowledge-graph.md)
