---
type: Domain Entity
title: Prepared Agent Context
description: A prebuilt bounded Markdown context plus a checksum manifest for an agent task.
tags:
  - agents
  - manifest
  - retrieval
implementation_paths:
  - pkg/retrieval/prepare.go
---

## Definition

A prepared context directory contains one Markdown file per named query and `manifest.json` with the source bundle ID, source concept IDs, per-file SHA-256 checksums, bounds, and truncation state.

## Relationships

- Enforced by: [Interprocess Target Lock](/components/interprocess-target-lock.md)
- Uses: [Bounded Retrieval Engine](/components/bounded-retrieval-engine.md)
