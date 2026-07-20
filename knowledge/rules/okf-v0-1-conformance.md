---
type: Business Rule
title: OKF v0.1 Conformance
description: Standard validation accepts the permissive Open Knowledge Format v0.1 consumption model.
tags:
  - conformance
  - okf
  - specification
specification: Open Knowledge Format 0.1 Draft
---

## Statement

Every non-reserved Markdown concept must have parseable YAML frontmatter and a non-empty `type`. `index.md` and `log.md` are optional reserved files. Standard consumers must tolerate unknown types and metadata, broken cross-links, and missing indexes; these are warnings rather than rejection conditions.

## Producer Guidance

Generated concept links use absolute bundle-relative Markdown. Indexes have no frontmatter and enumerate entries under headings. Citations use a final numbered `Citations` section when claims rely on external material.

## Relationships

- Applies to: [Knowledge Bundle](/concepts/knowledge-bundle.md)
- Enforced by: [Deterministic Knowledge Generator](/components/deterministic-generator.md)
- Enforced by: [OKF Parser and Store](/components/parser-store.md)

## Citations

[1] [Open Knowledge Format v0.1 specification](<https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md>): Defines bundle structure, frontmatter, links, indexes, logs, citations, and conformance.
