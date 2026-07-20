---
type: Architecture Decision
title: Fail Fast on Writer Contention
description: A competing process receives a retryable generation failure instead of waiting indefinitely or racing filesystem replacement.
tags:
  - architecture
  - concurrency
  - locks
---

## Decision

Acquire target locks non-blockingly. The lock winner completes its validated atomic replacement; every contender returns exit 5 with `GENERATION_FAILED`, leaves active output untouched, and may retry.

## Relationships

- Applies to: [Interprocess Target Lock](/components/interprocess-target-lock.md)
- Enforced by: [Single Writer Per Target](/rules/single-writer-per-target.md)
