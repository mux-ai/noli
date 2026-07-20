---
type: Domain Entity
title: Concept Document
description: One UTF-8 Markdown knowledge unit with YAML frontmatter and a structured body.
tags:
  - frontmatter
  - markdown
  - okf
recommended_frontmatter:
  - title
  - description
  - resource
  - tags
  - timestamp
required_frontmatter:
  - type
---

## Definition

Every non-reserved `.md` file is a concept. Its concept ID is its bundle-relative path without `.md`. Unknown frontmatter fields are producer extensions and must survive parser round trips.

## Relationships

- Applies to: [Knowledge Bundle](/concepts/knowledge-bundle.md)
- Follows: [OKF v0.1 Conformance](/rules/okf-v0-1-conformance.md)
