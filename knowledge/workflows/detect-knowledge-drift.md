---
type: Workflow
title: Detect Knowledge Drift
description: Find knowledge that no longer matches its concept source or repository changes that nobody documented.
tags:
  - drift
  - maintenance
  - validation
---

## Steps

1. Run `noli drift --config noli.yaml` (read-only; nothing is written).
2. Inspect `bundle`: added, changed, or removed IDs mean the active knowledge diverged from `.noli/concepts.yaml`, usually through hand edits.
3. Inspect `undocumented_files`: repository files changed since the last knowledge-touching commit without a knowledge update.
4. Resolve bundle drift by updating the concept source and re-applying generation; resolve undocumented files by authoring the missing concepts.
5. Exit code 4 signals drift, so the command can gate commits and CI.

## Relationships

- Follows: [Update and Regenerate Knowledge](/workflows/update-and-regenerate-knowledge.md)
- Uses: [Deterministic Knowledge Generator](/components/deterministic-generator.md)
- Uses: [Noli CLI](/components/noli-cli.md)
