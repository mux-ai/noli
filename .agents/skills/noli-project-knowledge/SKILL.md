---
name: noli-project-knowledge
description: Use Noli's noli CLI to retrieve bounded, source-traceable project knowledge before implementing features, fixing bugs, or answering questions about domain rules, entities, components, workflows, or architecture decisions.
---

# Noli project knowledge

This project stores structured knowledge as OKF Markdown documents. Query
them with the `noli` CLI instead of reading the files directly; results are
bounded, deterministic JSON with source traceability.

Find the knowledge root (usually `knowledge/`) and the optional `noli.yaml`
in the repository root.

## First-run protocol: let the developer choose

Before using Noli on the first task in a repository, determine its state:

1. `noli.yaml` or `knowledge/` exists in the repository root → Noli is
   enabled; use it as described below. Never ask.
2. `.noli/disabled` exists → the developer opted out; work normally and do
   not mention Noli again.
3. If no Noli namespace exists, accept `okf.yaml` or `.okf/disabled` as a
   legacy migration fallback; the Noli namespace always wins when both exist.
4. Neither exists → ask the developer exactly one question:
   *"This project has Noli installed but no project knowledge base yet.
   Initialize one so I can retrieve grounded project knowledge? (yes/no)"*
   - **yes** → run the bootstrap below, then use Noli from now on.
   - **no** → record the choice so nobody asks again, then work normally:

     ```bash
     mkdir -p .noli && echo "developer opted out" > .noli/disabled
     ```

### Bootstrap (only after the developer says yes)

1. Copy `noli-starter.yaml` (shipped next to this skill) to the repository
   root as `noli.yaml` and set `project.name`.
2. Copy `noli-starter-concepts.yaml` (also next to this skill) to
   `.noli/concepts.yaml`.
3. Generate the bundle and confirm health:

   ```bash
   noli generate --config noli.yaml --apply --format json
   noli validate --root knowledge --mode project --config noli.yaml --format json
   ```

   Expect `"valid": true` and exit code 0.
4. Immediately author real knowledge for the project (next section) and
   replace the starter placeholders.

## Authoring and updating knowledge

`.noli/concepts.yaml` is the single source of truth. Never edit files under
`knowledge/` by hand; describe concepts in the YAML and let Noli
render the Markdown and the knowledge graph deterministically.

Whenever you design a new project, feature, or significant change, capture
the design as concepts BEFORE implementing code:

1. Derive the knowledge from the task: domain entities (`Domain Entity`),
   invariants and constraints (`Business Rule`), services and adapters
   (`Application Component`), step sequences (`Workflow`), and technology
   or structure choices (`Architecture Decision`).
2. Append them to `.noli/concepts.yaml`. Give every concept a `title`, a
   `description`, at least one section, and `relationships` (predicates:
   `applies-to`, `depends-on`, `enforced-by`, `uses`, `follows`) pointing
   at other concepts by title — the relationships become the knowledge
   graph.
3. Preview, apply, and verify:

   ```bash
   noli generate --config noli.yaml --dry-run --format json # inspect the diff
   noli generate --config noli.yaml --apply --format json
   noli validate --root knowledge --mode project --config noli.yaml --format json
   ```

4. Then implement the code, retrieving from the fresh knowledge as you go,
   and treat the retrieved rules as binding.
5. When the implementation reveals new facts (edge cases, renamed
   components, new rules), update `.noli/concepts.yaml` and re-apply so the
   knowledge never drifts from the code.

Example: for the prompt "build a short URL generator with Python", author
concepts such as `Short Link` and `Slug` (entities), `Slug Uniqueness` and
`Link Expiry` (rules), `Shortener Service` and `Link Store` (components),
`Shorten URL` and `Resolve URL` (workflows), plus a storage-choice
decision — related through `applies-to`/`enforced-by`/`uses` — before
writing the first line of Python.

Prefer editing `knowledge/` Markdown directly instead? Then delete the
`generation:` section from `noli.yaml` first; a later `--apply` would
otherwise replace hand edits with the rendered concepts.

## Retrieve context for a task

```bash
noli retrieve --root knowledge --query "$TASK" \
  --search-limit 5 --max-hops 1 --max-documents 8 --max-characters 14000 \
  --direction both --format json
```

Read `data.context` (Markdown). Every document included is listed in
`data.sources` with its ID, seed flag, score, distance, and relationship.
Restrict types with `--types "Business Rule,Domain Entity"` when the task
is narrow.

## Other operations

```bash
noli status   --root knowledge --format json               # bundle overview
noli search   --root knowledge --query "..." --format json # ranked hits only
noli get      --root knowledge --id <document-id> --format json
noli graph    --root knowledge --id <document-id> --direction both --format json
noli validate --root knowledge --mode standard --format json
```

## Contract

- Exit 0 plus `"ok": true` is success; JSON is always on stdout only.
- Errors carry a stable `error.code` (for example `DOCUMENT_NOT_FOUND`,
  `KNOWLEDGE_NOT_FOUND`, `INVALID_ARGUMENT`).
- Exit codes: 2 invalid arguments, 3 loading failure, 4 validation failure,
  5 generation failure, 6 unsafe path, 7 internal.
- Never edit generated knowledge without running
  `noli validate` afterwards; treat retrieved business rules as binding.
