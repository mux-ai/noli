---
type: Business Rule
title: Single Writer Per Target
description: At most one process may replace a project bundle or prepared-context output at a time.
tags:
  - atomicity
  - concurrency
  - writes
---

## Statement

A writer must acquire the target lock before reading generation inputs or replacing output. Contenders fail safely with `GENERATION_FAILED` and may retry. Each attempt owns unique staging and backup paths, and rollback preserves the last active tree.

## Relationships

- Applies to: [Deterministic Knowledge Generator](/components/deterministic-generator.md)
- Applies to: [Prepared Agent Context](/concepts/prepared-agent-context.md)
- Enforced by: [Interprocess Target Lock](/components/interprocess-target-lock.md)
