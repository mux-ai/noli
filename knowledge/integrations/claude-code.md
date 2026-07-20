---
type: Agent Integration
title: Claude Code Integration
description: Project-local or user-global guidance, skill, and noli-context command for retrieving bounded project knowledge in Claude Code.
tags:
  - agents
  - claude
  - integration
implementation_paths:
  - integrations/claude
---

## Usage

Install `CLAUDE.md`, the `/noli-context` command, and the shared Noli skill into a target repository, or pass `--global` to install them below `CLAUDE_CONFIG_DIR` for the current user. Global installation preserves existing instructions and applies the first-run choice in every undecided repository. Claude invokes the local CLI and grounds answers in returned source concept IDs.

## Relationships

- Enforced by: [Agent Global First-Run Choice](/rules/agent-global-first-run-choice.md)
- Follows: [Retrieve Knowledge Before Coding](/workflows/retrieve-before-coding.md)
- Uses: [Noli CLI](/components/noli-cli.md)
