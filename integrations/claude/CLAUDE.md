# Noli project knowledge (Claude Code integration)

This repository has Noli installed. Project knowledge, when
enabled, lives in OKF Markdown documents queried through the local `noli`
CLI. Do not read the knowledge files directly; use the CLI so context stays
bounded and source-traceable.

## First: check whether Noli is enabled here

- `noli.yaml` or `knowledge/` exists → Noli is enabled; follow the rest of
  this file.
- `.noli/disabled` exists → the developer opted out; ignore the rest of this
  file and work normally.
- If no Noli namespace exists, legacy `okf.yaml` and `.okf/disabled` are
  accepted as migration fallbacks.
- Neither → ask the developer one direct question: *"Initialize a Noli
  knowledge base for this project? (yes/no)"*. On **yes**, follow the
  bootstrap steps in `.claude/skills/noli-project-knowledge/SKILL.md`
  (starter config `noli-starter.yaml` sits next to it). On **no**, run
  `mkdir -p .noli && echo "developer opted out" > .noli/disabled` and work
  normally from then on.

## When Noli is enabled

Before implementing a task, retrieve knowledge for it:

```bash
noli retrieve --root knowledge --query "<task summary>" \
  --search-limit 5 --max-hops 1 --max-documents 8 --max-characters 14000 \
  --direction both --format json
```

`data.context` is authoritative background; `data.sources` lists every
included document with its ID, seed flag, and relationship. Inspect single
documents with `noli get --root knowledge --id <id> --format json`.

Success is exit code 0 with `"ok": true`. Error envelopes carry a stable
`error.code`; exit codes are 2 invalid arguments, 3 loading failure,
4 validation failure, 5 generation failure, 6 unsafe path, 7 internal.

**Author knowledge before code.** When you design a new project, feature,
or significant change, first capture it as concepts in
`.noli/concepts.yaml` (entities, business rules, components, workflows,
decisions, with relationships between them), then run
`noli generate --config noli.yaml --apply --format json` followed by
`noli validate --mode project --config noli.yaml --root knowledge
--format json`. Only then implement, retrieving from the fresh knowledge.
Update the concepts file and re-apply whenever the implementation reveals
new facts. The full authoring workflow lives in
`.claude/skills/noli-project-knowledge/SKILL.md`.

Use the `/noli-context` command for one-step retrieval.
