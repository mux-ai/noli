---
type: Agent Integration
title: Claude Code Integration
description: Repository guidance and a noli-context slash command for retrieving bounded project knowledge in Claude Code.
tags:
  - agents
  - claude
  - integration
implementation_paths:
  - integrations/claude
---

## Usage

Install `CLAUDE.md`, the `/noli-context` command, and the shared Noli skill into the target repository. Claude invokes the local CLI and grounds answers in returned source concept IDs.

## Relationships

- Follows: [Retrieve Knowledge Before Coding](/workflows/retrieve-before-coding.md)
- Uses: [Noli CLI](/components/noli-cli.md)
