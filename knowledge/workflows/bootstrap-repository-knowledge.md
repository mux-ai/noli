---
type: Workflow
title: Bootstrap Repository Knowledge
description: Initialize a repository-local, spec-compatible OKF knowledge bundle from structured concepts.
tags:
  - bootstrap
  - generation
  - validation
---

## Steps

1. Create `noli.yaml` and `.noli/concepts.yaml`.
2. Run `noli generate --config noli.yaml --dry-run` and inspect the diff.
3. Run `noli generate --config noli.yaml --apply`.
4. Run both standard and project validation.
5. Exercise a representative retrieval query and inspect its sources.

## Relationships

- Follows: [OKF v0.1 Conformance](/rules/okf-v0-1-conformance.md)
- Uses: [Deterministic Knowledge Generator](/components/deterministic-generator.md)
- Uses: [Project Configuration](/concepts/project-configuration.md)
- Uses: [Structured Concept Source](/concepts/structured-concept-source.md)
