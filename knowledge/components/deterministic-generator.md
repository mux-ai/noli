---
type: Application Component
title: Deterministic Knowledge Generator
description: The non-LLM renderer that converts structured concepts into a validated OKF bundle.
tags:
  - deterministic
  - generation
  - markdown
implementation_paths:
  - pkg/generator
---

## Responsibilities

Resolve canonical concept IDs and relationships, render stable frontmatter, Markdown sections, absolute bundle links, numbered citations, and frontmatter-free indexes, then validate before an atomic apply. Hand-authored conformant log files are preserved.

## Relationships

- Enforced by: [Interprocess Target Lock](/components/interprocess-target-lock.md)
- Enforced by: [Generated Knowledge Source of Truth](/rules/generated-knowledge-source-of-truth.md)
- Follows: [Markdown and YAML Knowledge Storage](/decisions/markdown-yaml-storage.md)
- Uses: [Structured Concept Source](/concepts/structured-concept-source.md)
