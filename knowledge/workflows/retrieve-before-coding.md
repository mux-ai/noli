---
type: Workflow
title: Retrieve Knowledge Before Coding
description: Ground an implementation task in bounded project knowledge before editing code.
tags:
  - agents
  - implementation
  - retrieval
---

## Steps

1. Describe the concrete coding task as the retrieval query.
2. Run `noli retrieve` with explicit graph and context bounds.
3. Read `data.context` and verify `data.sources`.
4. Treat retrieved business rules and architecture decisions as constraints.
5. Re-query after the task reveals a more precise concept or component.

## Relationships

- Enforced by: [Bounded Deterministic Retrieval](/rules/bounded-deterministic-retrieval.md)
- Uses: [Bounded Retrieval Engine](/components/bounded-retrieval-engine.md)
- Uses: [Noli CLI](/components/noli-cli.md)
