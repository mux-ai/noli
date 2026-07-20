# Noli project knowledge (Codex integration)

This repository has Noli installed. Project knowledge, when
enabled, lives in OKF Markdown documents queried through the local `noli`
CLI. Do not read the knowledge files directly; use the CLI so context stays
bounded and source-traceable.

First, check whether Noli is enabled here: if `noli.yaml` or `knowledge/`
exists, it is enabled — follow the rest of this file. If `.noli/disabled`
exists, the developer opted out — ignore the rest of this file and work
normally. When no Noli namespace exists, accept legacy `okf.yaml` and
`.okf/disabled` as migration fallbacks. If neither exists, ask the developer
one direct question: *"Initialize a Noli knowledge base for this project?
(yes/no)"*. On yes,
follow the bootstrap steps in
`.agents/skills/noli-project-knowledge/SKILL.md` (starter config
`noli-starter.yaml` sits next to it). On no, run
`mkdir -p .noli && echo "developer opted out" > .noli/disabled` and work
normally from then on.

When Noli is enabled, run this before implementing a task:

```bash
noli retrieve --root knowledge --query "<task summary>" \
  --search-limit 5 --max-hops 1 --max-documents 8 --max-characters 14000 \
  --direction both --format json
```

Use `data.context` as authoritative background and cite `data.sources`
document IDs in your reasoning. Inspect single documents with
`noli get --root knowledge --id <id> --format json` and neighborhoods with
`noli graph --root knowledge --id <id> --direction both --format json`.

Success is exit code 0 with `"ok": true`; error envelopes carry a stable
`error.code`.

Author knowledge before code: when designing a new project, feature, or
significant change, first capture it as concepts in `.noli/concepts.yaml`
(entities, business rules, components, workflows, decisions, with
relationships between them), then run
`noli generate --config noli.yaml --apply --format json` and
`noli validate --root knowledge --mode project --config noli.yaml
--format json`. Only then implement, retrieving from the fresh knowledge,
and re-apply the concepts whenever the implementation reveals new facts.

The full operation reference and authoring workflow live in
`.agents/skills/noli-project-knowledge/SKILL.md`.
