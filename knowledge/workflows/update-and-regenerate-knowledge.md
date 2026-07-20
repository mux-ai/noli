---
type: Workflow
title: Update and Regenerate Knowledge
description: Keep the structured concept source, generated graph, and implementation decisions synchronized.
tags:
  - generation
  - maintenance
  - validation
---

## Steps

1. Add or revise concepts, sections, metadata, and relationships in `.noli/concepts.yaml`.
2. Run a dry-run and review added, changed, and removed concept IDs.
3. Apply the bundle under the project write lock.
4. Validate in project mode.
5. Retrieve the changed topic and verify the expected graph neighborhood.

## Relationships

- Enforced by: [Generated Knowledge Source of Truth](/rules/generated-knowledge-source-of-truth.md)
- Uses: [Deterministic Knowledge Generator](/components/deterministic-generator.md)
- Uses: [Interprocess Target Lock](/components/interprocess-target-lock.md)
