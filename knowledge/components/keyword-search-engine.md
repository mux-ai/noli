---
type: Application Component
title: Keyword Search Engine
description: Deterministic integer-scored keyword search over concept metadata and bodies.
tags:
  - deterministic
  - ranking
  - search
implementation_paths:
  - pkg/search
---

## Responsibilities

Tokenize searchable records, score title, description, tag, producer metadata, body, and phrase matches with fixed integer weights, then sort by descending score and ascending concept ID.

## Relationships

- Applies to: [Concept Document](/concepts/concept-document.md)
- Uses: [Bounded Retrieval Engine](/components/bounded-retrieval-engine.md)
