---
type: Application Component
title: Todo Repository
description: Persistence adapter storing todo items.
tags: [repository, persistence]
---

## Responsibility

Stores and loads todo items. No domain logic lives here; validation happens
in domain services before any write.

## Relationships

- Depends on: [Todo Item](/concepts/todo-item.md)

## Notes

Demonstration data for Noli examples.
