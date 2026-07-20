---
type: Business Rule
title: Generated Knowledge Source of Truth
description: When generation is configured, agents edit structured concepts rather than generated Markdown.
tags:
  - agents
  - generation
  - source-of-truth
---

## Statement

Change `.noli/concepts.yaml`, preview the deterministic diff, apply, and validate. Do not hand-edit generated concept documents because a later apply will replace them. Optional hand-authored `log.md` files are the reserved exception and survive generation.

## Relationships

- Applies to: [Structured Concept Source](/concepts/structured-concept-source.md)
- Enforced by: [Update and Regenerate Knowledge](/workflows/update-and-regenerate-knowledge.md)
