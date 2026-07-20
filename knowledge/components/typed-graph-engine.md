---
type: Application Component
title: Typed Graph Engine
description: Cycle-safe directed graph traversal retaining distance, predecessor, relationship, and seed rank.
tags:
  - graph
  - relationships
  - traversal
implementation_paths:
  - pkg/graph
---

## Responsibilities

Traverse outgoing, incoming, or both directions within document and hop bounds. Preserve ranked seed order and deterministic graph-only ordering without importing the OKF document package.

## Relationships

- Applies to: [Knowledge Graph](/concepts/knowledge-graph.md)
- Follows: [Typed Relationship Interpretation](/decisions/typed-relationship-interpretation.md)
