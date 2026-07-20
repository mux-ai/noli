---
type: Workflow
title: Complete Task Workflow
description: Steps for marking a tracked task as done.
tags: [workflow, completion]
---

## Steps

1. Load a [Todo Item](/concepts/todo-item.md) by identifier.
2. Check its [Task Status](/concepts/task-status.md).
3. Apply [Complete Task](/rules/complete-task.md).
4. Persist it via [Todo Repository](/components/todo-repository.md).

## Relationships

- Follows: [Complete Task](/rules/complete-task.md)
- Uses: [Todo Service](/components/todo-service.md)

## Notes

Demonstration data for Noli examples.
