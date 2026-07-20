---
type: Business Rule
title: Repository Path Containment
description: Local tools must not read, write, or execute through paths that escape the configured repository.
tags:
  - containment
  - paths
  - security
---

## Statement

Resolve real paths, reject traversal and symlink escapes, disallow NUL and unsafe path components, and exclude sensitive directories such as `.git`, credentials, secrets, dependencies, and build output.

## Relationships

- Applies to: [Deterministic Knowledge Generator](/components/deterministic-generator.md)
- Applies to: [OKF Parser and Store](/components/parser-store.md)
- Applies to: [Pi Coding Agent Integration](/integrations/pi-coding-agent.md)
