---
type: Workflow
title: Run Acceptance Gates
description: Verify implementation, conformance, protocol, integrations, race safety, and deterministic behavior before completion.
tags:
  - acceptance
  - race
  - testing
implementation_paths:
  - scripts/gate.sh
---

## Steps

1. Run focused tests for changed packages.
2. Run `scripts/gate.sh` for tests, vet, builds, dependency rules, fixtures, and Pi loader checks.
3. Run the race detector and contention tests for write-path changes.
4. Validate the repository knowledge bundle in standard and project modes.
5. Exercise representative retrieval and inspect source IDs.

## Relationships

- Uses: [Frozen JSON Protocol](/components/json-protocol.md)
- Uses: [Knowledge Bundle](/concepts/knowledge-bundle.md)
