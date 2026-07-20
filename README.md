# Noli

**Your friendly knowledge-format builder.**

Noli is a local-first tool that gives coding agents bounded, deterministic,
source-traceable access to project knowledge stored as
[Open Knowledge Format](https://github.com/GoogleCloudPlatform/knowledge-catalog/blob/main/okf/SPEC.md)
(OKF v0.1) Markdown documents. No MCP server, no background process, no
network transport, no vector database — one Go binary, one JSON protocol.

The repository also contains Noli's extended ingestion pipeline (`cmd/noligen`
plus `internal/`), which now runs on the same public engine packages.

## Installation

```bash
sh scripts/install-local.sh                 # user-local CLI in ~/.local/bin
sh integrations/codex/install.sh --global  # optional: enable Noli in every Codex repository
# or manually:
go build -buildvcs=false -o bin/noli ./cmd/noli
```

The Codex `--global` mode is user-global, not system-wide. It installs the
skill under `~/.agents/skills`, adds an idempotent managed block to the active
global Codex guidance under `${CODEX_HOME:-~/.codex}`, and preserves existing
instructions. New repositories ask once whether to initialize Noli; enabled
and opted-out repositories do not ask again. Use
`integrations/codex/install.sh <repository>` for a project-local integration.

Requires Go 1.22+. The Pi integration tests require Node.js 22+ and an
installed Pi 0.78-compatible CLI for the real-loader check.

### Namespace migration

Noli is the primary namespace for new work: use `noli`, `noli.yaml`,
`.noli/`, `noli-agent-queries.yaml`, `NOLI_*` environment variables,
`/noli-context`, and the five `noli_*` Pi tools. The `okf` and `okfgen`
executables and former environment variables remain deprecated compatibility
aliases. “OKF” still means the Open Knowledge Format standard and therefore
remains in format, bundle, conformance, parser, and Go SDK terminology.

## Quick start (Todo example)

```bash
noli status   --root examples/todo-app/knowledge --format json
noli retrieve --root examples/todo-app/knowledge \
  --query "Implement the CompleteTodo use case" \
  --types "Business Rule,Domain Entity,Application Component,Architecture Decision" \
  --search-limit 5 --max-hops 1 --max-documents 8 --max-characters 14000 \
  --direction both --format json
```

The retrieval returns a Markdown context assembled from the business rule
(seed) plus its typed graph neighborhood, with every selected source listed.

## This repository's knowledge graph

Noli is also documented as its own OKF v0.1 bundle under
`knowledge/`. Its deterministic source is `.noli/concepts.yaml`, governed by
the root `noli.yaml`; `noli-agent-queries.yaml` contains reusable queries for
implementation, conformance, retrieval/protocol, concurrent writers, and the
Claude Code/Codex/Pi integrations.

```bash
noli status --root knowledge --format json
noli retrieve --root knowledge \
  --query "Coordinate three coding agents safely during knowledge generation" \
  --search-limit 6 --max-hops 2 --max-documents 12 \
  --max-characters 18000 --direction both --format json
```

Update the graph through `.noli/concepts.yaml`, then preview, apply, and
validate it:

```bash
noli generate --config noli.yaml --dry-run --format json
noli generate --config noli.yaml --apply --format json
noli validate --root knowledge --mode project --config noli.yaml --format json
```

Generated concept documents and indexes should not be edited directly.
`knowledge/log.md` is the intentional hand-authored exception permitted by
the OKF specification and is preserved across generation.

The OKF boundary is explicit: only directories configured as a
`knowledge.root` are knowledge bundles. Files such as `README.md`,
`docs/*.md`, integration guidance, skills, and example source inputs are
ordinary project documentation and must not be treated as graph concepts or
given synthetic OKF frontmatter.

## CLI commands

| Command | Purpose |
|---|---|
| `status` | Bundle overview: counts, types, bundle checksum |
| `list` | Document summaries (navigation/log excluded by default) |
| `search` | Ranked keyword hits with integer scores |
| `retrieve` | Bounded context: search seeds + graph expansion |
| `get` | One document with metadata, typed links, body |
| `graph` | Bounded relationship neighborhood of one document |
| `validate` | `--mode standard` or `--mode project --config noli.yaml` |
| `generate` | Deterministic bundle generation; `--dry-run` or `--apply` |
| `prepare-agent-context` | Prebuilt context files + manifest for agent runs |

All commands emit a single JSON line on stdout (`--format json` is the
default and only format). Successful commands leave stderr empty.

## JSON protocol

Frozen in `docs/PROTOCOL.md`; golden fixtures live in
`pkg/protocol/testdata/fixtures/` and are checksum-locked by
`docs/fixtures.sha256`.

Envelope:

```json
{"ok": true,  "command": "status", "version": 1, "data": {}}
{"ok": false, "command": "get", "version": 1,
 "error": {"code": "DOCUMENT_NOT_FOUND", "message": "...", "details": {}}}
```

Exit codes: `0` success, `2` invalid arguments, `3` loading failure,
`4` validation failure, `5` generation failure, `6` unsafe path,
`7` internal. Stable error codes: `INVALID_ARGUMENT`,
`KNOWLEDGE_NOT_FOUND`, `DOCUMENT_NOT_FOUND`, `UNSAFE_PATH`, `PARSE_ERROR`,
`INVALID_FRONTMATTER`, `VALIDATION_FAILED`, `GENERATION_FAILED`,
`CONTEXT_LIMIT_TOO_SMALL`, `INTERNAL_ERROR`.

`validate` reports an invalid bundle as a success envelope with exit code 4;
error-envelope `VALIDATION_FAILED` is reserved for operations that refuse to
proceed (for example a failed `generate --apply`, which rolls back).

## OKF v0.1 conformance

The toolkit reads and writes bundles conforming to the Open Knowledge
Format v0.1 specification published by Google Cloud on 12 June 2026.

Producer rules honored:

- Every non-reserved `.md` document carries parseable YAML frontmatter with
  a non-empty `type` (conformance rules 1 and 2).
- The recommended fields `title`, `description`, `resource`, `tags`, and
  `timestamp` are first-class; unknown frontmatter keys are preserved
  verbatim on read and write.
- Reserved filenames are `index.md` and `log.md` at any level. Index files
  are written **without frontmatter** and list concepts as
  `* [Title](/path.md) - description` (§6). Logs are never machine-written
  because §7 requires ISO 8601 `YYYY-MM-DD` entry headings and generation
  has no clock; hand-authored logs are preserved across `--apply`.
- Generated links use the absolute, bundle-relative `/path.md` form §5.1
  recommends; relative links are still read correctly.

Consumer rules honored (§9): a bundle is never rejected for missing
optional fields, unknown `type` values, unknown frontmatter keys, broken
cross-links, or missing `index.md`. Those surface as **warnings** in
`validate --mode standard`, whose only errors are parse failures and a
missing `type`. Stricter checks live in `--mode project`, which is opt-in
local policy layered on top of the spec.

Toolkit-specific extension: relationship phrases in list items
(`- Applies to: [X](/concepts/x.md)`) are normalized into typed graph
predicates. The spec treats links as untyped, so this is an interpretation
of surrounding prose; unrecognized phrasing falls back to `links-to`, and
the documents stay plain conformant Markdown.

## Public Go SDK

```go
import "noli/pkg/okf"

store, err := okf.Load("knowledge")
document, ok := store.Get("rules/complete-task")
documents := store.List(okf.ListOptions{Types: []string{"Business Rule"}})
results := store.Search("task completion", okf.SearchOptions{Limit: 10})
retrieved, err := store.Retrieve("Implement the CompleteTodo use case", okf.RetrieveOptions{})
view, err := store.GraphView("rules/complete-task", okf.GraphOptions{})
report := okf.Validate("knowledge", okf.ValidationOptions{})
```

Package layout (dependency direction is enforced by `scripts/gate.sh`):

```text
pkg/graph      IDs and typed edges only; no OKF imports
pkg/search     package-independent records, integer scoring
pkg/retrieval  seeds + expansion + bounded context; no OKF imports
pkg/okf        documents, parser, Store, validation
pkg/generator  strict noli.yaml, deterministic generation
pkg/protocol   frozen JSON DTOs only
```

The Store is immutable; accessors return deep copies. `BundleID()` is a
SHA-256 over sorted document IDs and normalized bytes.

## Agent integrations

Hands-on walkthrough for Claude Code + Codex side by side:
[`integrations/README.md`](integrations/README.md).

Thin local CLI wrappers only — no services:

- `integrations/shared/SKILL.md` — generic agent skill.
- `integrations/codex/` — AGENTS.md plus `install.sh` to copy guidance into
  a repository.
- `integrations/claude/` — CLAUDE.md plus a `/noli-context` command.
- `integrations/pi/` — verified Pi 0.78 TypeScript adapter and installer:
  `spawn` with `shell: false`,
  five allowlisted read operations, repository containment with symlink
  resolution, bounded streamed output, immediate kill on timeout or
  overflow. `npm --prefix integrations/pi test`.

## Knowledge generation

`noli generate --config noli.yaml` renders structured concept inputs
(`generation.concepts` inline or `generation.concept_files` YAML/JSON) into
a complete bundle — concepts, directory indexes, root index, and a static
log. No LLM is involved and no generated timestamps exist, so identical
inputs produce identical bytes.

- `--dry-run` writes only `.noli/preview` and reports
  added/changed/removed/unchanged against the active knowledge.
- `--apply` builds a temporary sibling, validates it with the project rules,
  and atomically replaces the active root; any failure rolls back
  byte-for-byte.
- With no configured inputs, an existing bundle is re-rendered as
  normalized source (BOM stripped, line endings normalized).

## Security model

- No shell execution anywhere in the SDK; the CLI reads only the supplied
  roots and configs.
- Paths are containment-checked after symlink resolution; NUL bytes,
  traversal, backslashes, and sensitive components (`.git`, `.env*`,
  `secrets`, `credentials`, keys, `node_modules`, `vendor`, `build`) are
  rejected.
- Bundle loading is bounded: 2 MiB per file, 64 MiB per bundle, 10000
  documents by default.
- Active knowledge is never modified without `--apply`; all directory
  writes go through temporary siblings with validation, rename, rollback,
  and cleanup.
- Generation and prepared-context replacement are serialized by target-specific
  interprocess locks. A competing agent fails safely and can retry; unique
  staging and backup paths prevent one process from deleting another's work.
- Prepared-context outputs are replaced only when empty or previously
  prepared (manifest present).
- Public packages contain no `panic` and no global mutable registries.

## Limitations

- Keyword search only (deterministic integer scoring); no embeddings.
- `retrieve` cannot request "seeds only": hop count 0 selects the frozen
  default of 1.
- Relationship phrases recognize five predicates (`applies-to`,
  `enforced-by`, `depends-on`, `uses`, `follows`); other prose stays
  `links-to`.
- Pi compatibility is loader-tested against the installed 0.78 API; future
  Pi API upgrades should rerun `npm --prefix integrations/pi test`.
- On non-Linux hosts, interprocess locking uses an atomic lock-directory
  fallback. A process crash can leave a stale `.d` lock that must be removed
  only after confirming no writer is still active.
- Noli's extended extraction pipeline (`cmd/noligen`) still uses its own
  workspace-oriented document model under `internal/`; it shares the public
  graph/search/retrieval engine but not `pkg/okf`.

## Development

```bash
scripts/gate.sh            # phase gates: tests, vet, dependency rules, contract lock
go test ./...              # all Go tests
npm --prefix integrations/pi test
```
