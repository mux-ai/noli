---
type: Application Component
title: Interprocess Target Lock
description: A process-safe non-blocking lock preventing concurrent replacement of the same generated output.
tags:
  - concurrency
  - filesystem
  - locking
implementation_paths:
  - pkg/internal/targetlock
linux_mechanism: flock
portable_fallback: atomic-directory
---

## Responsibilities

Serialize generation per project and prepared-context replacement per output target. Linux uses kernel-released advisory locks; other platforms fail closed with atomic lock directories.

## Relationships

- Enforced by: [Single Writer Per Target](/rules/single-writer-per-target.md)
- Follows: [Fail Fast on Writer Contention](/decisions/fail-fast-writer-contention.md)
