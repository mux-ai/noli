---
type: Domain Entity
title: Structured Concept Source
description: The deterministic YAML input from which generated concept Markdown and graph links are rendered.
tags:
  - generation
  - source-of-truth
  - yaml
implementation_paths:
  - .noli/concepts.yaml
  - pkg/generator/concept.go
---

## Definition

`.noli/concepts.yaml` records concept identity, type, title, description, producer metadata, sections, relationships, and citations. It is the source of truth whenever `noli.yaml` has a `generation` section.

## Relationships

- Applies to: [Concept Document](/concepts/concept-document.md)
- Enforced by: [Generated Knowledge Source of Truth](/rules/generated-knowledge-source-of-truth.md)
- Uses: [Deterministic Knowledge Generator](/components/deterministic-generator.md)
