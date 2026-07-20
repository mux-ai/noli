---
type: Business Rule
title: Complete Task
description: Business rule for the CompleteTodo use case.
tags: [completetodo, completion]
---

## Statement

To implement the CompleteTodo use case: a task may only be completed while it
is `in-progress`, completing an already `done` task is idempotent, and a
completed task becomes immutable.

## Relationships

- Applies to: [Todo Item](/concepts/todo-item.md)
- Depends on: [Task Status](/concepts/task-status.md)
- Enforced by: [Todo Service](/components/todo-service.md)
- Uses: [Todo Repository](/components/todo-repository.md)

## Related decisions

- [Domain Validation Layer](/decisions/domain-validation-layer.md)

## Notes

Demonstration data for Noli examples.
