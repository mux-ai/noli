# Noli — frozen OKF protocol contracts

Status: frozen at Phase 0 (2026-07-20). Changes after this point require
coordinator sign-off and fixture updates in the same change.

Protocol version: `1`

## 1. JSON envelope

All commands support `--format json`. JSON output goes only to stdout, ends
with exactly one trailing newline, and successful JSON commands leave stderr
empty. Diagnostics go only to stderr. Arrays in responses are never `null`;
empty collections serialize as `[]`.

Success:

```json
{"ok": true, "command": "<command>", "version": 1, "data": {…}}
```

Error:

```json
{"ok": false, "command": "<command>", "version": 1,
 "error": {"code": "<STABLE_CODE>", "message": "<human sentence>", "details": {…}}}
```

- `command` is the resolved subcommand name, or `""` when the command could
  not be resolved.
- `error.details` is an optional string-keyed object with stable keys per
  code (for example `{"id": "rules/missing"}` for `DOCUMENT_NOT_FOUND`).
  It is omitted when empty.
- Command handlers return exit codes; only `main` calls `os.Exit`.
- `--format json` is scanned before normal flag parsing so malformed
  JSON-mode invocations still receive a JSON error.
- Default output format is `json` (agent-first tool).

## 2. Exit codes

| Exit | Meaning |
|---:|---|
| 0 | Success |
| 2 | Invalid command or arguments |
| 3 | Knowledge/config/document loading failure |
| 4 | Validation failure |
| 5 | Generation failure |
| 6 | Unsafe path or containment failure |
| 7 | Unclassified internal failure |

## 3. Stable error codes and their exit codes

| Code | Exit |
|---|---:|
| `INVALID_ARGUMENT` | 2 |
| `CONTEXT_LIMIT_TOO_SMALL` | 2 |
| `KNOWLEDGE_NOT_FOUND` | 3 |
| `DOCUMENT_NOT_FOUND` | 3 |
| `PARSE_ERROR` | 3 |
| `INVALID_FRONTMATTER` | 3 |
| `VALIDATION_FAILED` | 4 |
| `GENERATION_FAILED` | 5 |
| `UNSAFE_PATH` | 6 |
| `INTERNAL_ERROR` | 7 |

Notes:

- `noli validate` reports an invalid knowledge base as a **success envelope**
  (`ok: true`) carrying the full report, with exit code 4 when `errors` is
  non-empty and exit code 0 otherwise (warnings alone do not fail).
  `VALIDATION_FAILED` as an error envelope is reserved for operations that
  refuse to proceed because validation failed (for example `generate --apply`
  rolling back).
- `CONTEXT_LIMIT_TOO_SMALL` is raised when even a single source header cannot
  fit into `--max-characters`.
- `noli drift` reports a drifted project as a **success envelope** with exit
  code 4 when `drifted` is true and 0 otherwise, mirroring `validate`. It is
  read-only: the concept source is rendered in memory and diffed against the
  active knowledge (`bundle`), and repository files changed since the last
  commit touching the knowledge root, `.noli/`, `noli.yaml`, or a configured
  concept file are reported as `undocumented_files` with states `added`,
  `modified`, `deleted`, `renamed`, `copied`, or `untracked`. Without a
  usable git repository, `git` is `"unavailable"`, `baseline` is empty, and
  `undocumented_files` is `[]`; only bundle drift is detected. `drift`
  requires `--config`.

## 4. Commands and frozen defaults

Read-only commands: `status`, `list`, `search`, `retrieve`, `get`, `graph`,
`validate`, `drift`. Write commands: `generate`, `prepare-agent-context`,
`enable`, `disable`, `clean`.

`enable --dir <repository>` (default `.`) removes the `.noli/disabled` and
legacy `.okf/disabled` opt-out sentinels; when neither `noli.yaml` nor
`knowledge/` exists it writes the embedded starter configuration (with
`project.name` derived from the directory name) and `.noli/concepts.yaml`,
then generates and validates the bundle. `disable --dir <repository>`
writes `.noli/disabled` with `developer opted out`. Both are idempotent and
report `changed: false` when the state already matched; because every agent
integration reads the same files, one invocation switches the repository
for all coding agents.

`clean --dir <repository>` is two-phase: without `--force` it is a pure
preview that lists every existing Noli-related path (knowledge root,
`noli.yaml`, `.noli/`, the agent queries file, and the deprecated `okf`
counterparts) and deletes nothing; with `--force` it deletes exactly those
paths. Agents MUST show the developer the preview list and obtain explicit
confirmation before running `--force` — clean destroys developer-authored
knowledge.

| Flag | Default |
|---|---|
| `--format` | `json` |
| `search --limit` | `10` |
| `retrieve --search-limit` | `5` |
| `retrieve --max-hops` | `1` |
| `retrieve --max-documents` | `10` |
| `retrieve --max-characters` | `12000` |
| `retrieve --direction` | `both` |
| `graph --direction` | `both` |
| `graph --max-hops` | `1` |
| `validate --mode` | `standard` |
| `enable --dir` | `.` |
| `disable --dir` | `.` |

`--types` accepts a comma-separated list; matching is case-insensitive on the
trimmed type name and applies to both search seeds and expanded candidates.
`generate` requires exactly one of `--dry-run` and `--apply`.

## 5. Search scoring (integers only)

Per unique query token found in a field:

```text
title                       +5
description                 +3
tags                        +2
custom metadata             +2
body                        +1
exact normalized phrase     +1 per matching field
```

Normalized phrase = lowercase, whitespace collapsed to single spaces. Sort by
score descending, then document ID ascending. Navigation (index) and log
documents are excluded unless explicitly included.

## 6. Retrieval ordering

1. Search seeds: score descending, then ID.
2. Graph-only documents: distance ascending.
3. Within a distance: originating seed rank, relationship, then ID.
4. Seed documents always precede graph-only documents.
5. Include/exclude type filters apply to seeds and expanded candidates.
6. Deduplicate by canonical document ID.
7. Stop at maximum documents and maximum hops.
8. Add complete context sections while they fit.
9. A single selected document exceeding the remaining budget is truncated on
   rune boundaries only, with `truncated: true`.
10. If even a source header cannot fit, fail with `CONTEXT_LIMIT_TOO_SMALL`.

## 7. Graph directions and traversal records

```go
type Direction string

const (
    DirectionOutgoing Direction = "outgoing"
    DirectionIncoming Direction = "incoming"
    DirectionBoth     Direction = "both"
)
```

Every traversal record retains: `ID`, `distance`, `predecessor ID`,
`relationship predicate`, `seed rank`. Seed records have distance `0`, empty
predecessor, and empty relationship. Caller seed order is preserved exactly
(first occurrence wins); seeds are never re-sorted.

Ordinary Markdown links use predicate `links-to`. Recognized deterministic
relationship phrase normalizations:

| Phrase | Predicate |
|---|---|
| `Applies to` | `applies-to` |
| `Enforced by` | `enforced-by` |
| `Depends on` | `depends-on` |
| `Uses` | `uses` |
| `Follows` | `follows` |

Unknown prose remains `links-to`; semantics are never guessed.

## 8. Validation problem codes

Validation reports use their own problem-code namespace (extensible; these
are the initial frozen codes):

```text
MISSING_TYPE, UNKNOWN_TYPE, MISSING_METADATA, MISSING_SECTION, BROKEN_LINK,
DUPLICATE_ID, MISSING_CONFIDENCE, INVALID_CONFIDENCE, MISSING_CITATION,
MISSING_INDEX, EMPTY_INDEX, EMPTY_DOCUMENT, INVALID_LOG_HEADING,
WRONG_DIRECTORY, DUPLICATE_CONCEPT, UNSAFE_METADATA, LOW_CONFIDENCE
```

Every problem carries `code`, `document` (ID or `""` for bundle-level), and
`message`. Errors and warnings are separate arrays, each in stable order
(document ID, then code, then message). Confidence and citations are
validated only when the config requires them or the field is present.

### 8.1 Severity and OKF v0.1 conformance

The toolkit implements the Open Knowledge Format v0.1 specification
published by Google Cloud. Section 9 of that specification forbids
consumers from rejecting a bundle because of missing optional frontmatter
fields, unknown `type` values, unknown additional frontmatter keys, broken
cross-links, or missing `index.md` files.

Standard mode therefore reports these as **warnings**: `BROKEN_LINK`,
`MISSING_INDEX`, `EMPTY_INDEX`, `EMPTY_DOCUMENT`, `INVALID_LOG_HEADING`.
The only standard-mode errors are genuine parse failures and `MISSING_TYPE`
(conformance rule 2 makes a non-empty `type` a MUST on every non-reserved
document).

Project mode (`--mode project --config noli.yaml`) is opt-in local policy and
may escalate the structural warnings to errors, and adds its own codes
(`UNKNOWN_TYPE`, `WRONG_DIRECTORY`, `MISSING_METADATA`, `MISSING_SECTION`,
`MISSING_CITATION`, `DUPLICATE_CONCEPT`, `LOW_CONFIDENCE`,
`UNSAFE_METADATA`).

Reserved filenames (`index.md`, `log.md` at any level) are exempt from the
frontmatter requirement; section 6 states index files carry no frontmatter.
Reserved files that do carry frontmatter are parsed and tolerated.

## 9. Config schema outline (`noli.yaml`)

Strict parsing via `yaml.Decoder.KnownFields(true)`. Sections:

```text
version, project, knowledge, sources, concept_types, relationships,
retrieval, agent, security, generation (optional)
```

Paths resolve relative to the directory containing `noli.yaml`. Path rules:
no NUL bytes, no absolute paths where relative required, no `..`, no unsafe
backslashes; `EvalSymlinks` on existing ancestors before containment checks;
sensitive components rejected (`.git`, `.env`, `secrets`, `credentials`,
private keys, `node_modules`, `vendor`, `build`). Document IDs are relative
Markdown IDs, never filesystem paths.

The deprecated `okf` executable and former product namespace remain
compatibility inputs only. They emit the same protocol version and response
shapes. OKF remains the name of the Open Knowledge Format consumed by Noli.

## 10. Golden fixtures

`pkg/protocol/testdata/fixtures/success/*.json` — one per command success
shape. `pkg/protocol/testdata/fixtures/error/*.json` — one per stable error
code. Phase 4 protocol tests must marshal real response structs and compare
byte-for-byte (after JSON normalization) against these files.
