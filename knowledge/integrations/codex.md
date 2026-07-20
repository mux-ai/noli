---
type: Agent Integration
title: Codex Integration
description: Project-local or user-global guidance and a shared skill that make Codex resolve Noli state and retrieve project knowledge before implementation.
tags:
  - agents
  - codex
  - integration
implementation_paths:
  - integrations/codex
---

## Usage

Run the Codex installer against one repository for checked-in guidance, or pass `--global` to install the skill and managed first-run guidance for the current user across repositories. Global installation preserves existing Codex instructions and asks once in each repository that has neither an enabled nor disabled Noli state. Codex uses the same bounded local CLI operations in both modes.

## Relationships

- Enforced by: [Agent Global First-Run Choice](/rules/agent-global-first-run-choice.md)
- Follows: [Retrieve Knowledge Before Coding](/workflows/retrieve-before-coding.md)
- Uses: [Noli CLI](/components/noli-cli.md)
