---
type: Architecture Decision
title: Domain Validation Layer
description: Domain rules live in a dedicated validation layer.
tags: [architecture, validation]
---

## Decision

Domain rules are validated inside a dedicated layer owned by domain
services, never in controllers or repositories.

## Consequences

Rules stay testable in isolation and reusable across workflows. Storage
adapters remain free of business logic.

## Notes

Demonstration data for Noli examples.
