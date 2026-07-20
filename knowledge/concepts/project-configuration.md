---
type: Domain Entity
title: Project Configuration
description: The strict repository-local noli.yaml policy used for generation, validation, retrieval defaults, and paths.
tags:
  - configuration
  - generation
  - validation
implementation_paths:
  - pkg/generator/config.go
---

## Definition

`noli.yaml` selects the knowledge root, project concept taxonomy, permitted relationship predicates, retrieval defaults, agent query file, security exclusions, and deterministic concept sources. A legacy `okf.yaml` is accepted only when `noli.yaml` is absent. Unknown configuration fields are rejected.

## Relationships

- Applies to: [Knowledge Bundle](/concepts/knowledge-bundle.md)
- Enforced by: [Deterministic Knowledge Generator](/components/deterministic-generator.md)
