---
type: Application Component
title: Public Go SDK
description: The reusable Go facade for loading, querying, validating, and traversing OKF bundles.
tags:
  - api
  - go
  - sdk
implementation_paths:
  - pkg/okf
---

## Responsibilities

Expose immutable `Store` operations for get, list, search, retrieve, graph view, validation, and bundle identity without launching the CLI or depending on a network service.

## Relationships

- Uses: [Bounded Retrieval Engine](/components/bounded-retrieval-engine.md)
- Uses: [Keyword Search Engine](/components/keyword-search-engine.md)
- Uses: [OKF Parser and Store](/components/parser-store.md)
- Uses: [Typed Graph Engine](/components/typed-graph-engine.md)
