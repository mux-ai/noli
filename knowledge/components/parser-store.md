---
type: Application Component
title: OKF Parser and Store
description: The bounded parser and immutable in-memory representation of an OKF knowledge bundle.
tags:
  - parser
  - security
  - store
implementation_paths:
  - pkg/okf/parser.go
  - pkg/okf/store.go
  - pkg/okf/metadata.go
---

## Responsibilities

Parse UTF-8 Markdown and YAML frontmatter, preserve unknown metadata, enforce file and aggregate limits, exclude sensitive paths, extract safe local links, and expose deep-copy accessors.

## Relationships

- Applies to: [Knowledge Bundle](/concepts/knowledge-bundle.md)
- Enforced by: [Repository Path Containment](/rules/repository-path-containment.md)
- Follows: [OKF v0.1 Conformance](/rules/okf-v0-1-conformance.md)
