---
type: Application Component
title: Todo Service
description: Application service coordinating task operations.
tags: [service, application]
---

## Responsibility

Coordinates task creation, completion, and deletion. Validates domain rules
before persisting changes; controllers never talk to storage directly.

## Relationships

- Uses: [Todo Repository](/components/todo-repository.md)
- Follows: [Domain Validation Layer](/decisions/domain-validation-layer.md)

## Notes

Demonstration data for Noli examples.
