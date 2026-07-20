---
type: Workflow
title: Create Task
description: Steps for creating a new tracked task.
tags: [workflow, creation]
---

## Steps

1. Receive a title and an optional priority.
2. Validate input via [Task Title Validation](/rules/task-title-validation.md).
3. Build a [Todo Item](/concepts/todo-item.md) with status `open`.
4. Persist it via [Todo Repository](/components/todo-repository.md).

## Relationships

- Follows: [Task Title Validation](/rules/task-title-validation.md)
- Uses: [Todo Service](/components/todo-service.md)

## Notes

Demonstration data for Noli examples.
