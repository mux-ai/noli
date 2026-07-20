# Trying Noli with Claude Code, Codex, and Pi

Noli is your friendly knowledge-format builder. This guide walks through
wiring its stable `noli` CLI into Claude Code, Codex, and Pi
so all three agents pull bounded, source-traceable project knowledge instead of
guessing. Everything runs locally — one binary, no server, no network.

## 1. Prerequisites

- Go 1.22+ (to build the binary)
- Claude Code, Codex, and/or Pi CLI installed
- This repository checked out (referred to as `<noli-repo>` below)

## 2. Install the Noli binary

```bash
cd <noli-repo>
sh scripts/install-local.sh
# installs to ~/.local/bin/noli; override with NOLI_INSTALL_DIR=/some/bin
noli status --root examples/todo-app/knowledge --format json
```

You should see a one-line JSON envelope with `"ok": true` and
`"document_count": 18`. If the shell cannot find `noli`, add `~/.local/bin`
to your `PATH`.

## 3. Pick a playground

The fastest playground is the shipped Todo example. Treat
`<noli-repo>/examples/todo-app` as if it were your project repository:

```bash
cd <noli-repo>/examples/todo-app
noli retrieve --root knowledge \
  --query "Implement the CompleteTodo use case" \
  --types "Business Rule,Domain Entity,Application Component,Architecture Decision" \
  --search-limit 5 --max-hops 1 --max-documents 8 --max-characters 14000 \
  --direction both --format json
```

For your own project instead: create a `knowledge/` directory of OKF
Markdown documents (copy the Todo example structure), plus an `noli.yaml`
modeled on `examples/todo-app/noli.yaml`, then run
`noli validate --root knowledge --mode project --config noli.yaml` until it
reports `"valid": true`.

## 4. Wire up Claude Code

Install Claude Code guidance, the shared skill, and `/noli-context` into one
repository:

```bash
sh <noli-repo>/integrations/claude/install.sh <your-project>
```

If `CLAUDE.md` exists, the installer preserves it and asks you to merge the
Noli guidance manually. To install for every repository for the current user:

```bash
sh <noli-repo>/integrations/claude/install.sh --global
```

Global mode honors `CLAUDE_CONFIG_DIR`, preserves an existing user
`CLAUDE.md`, and installs the skill and command below that directory. Start a
new Claude Code session and try `/noli-context complete a todo item`.

## 5. Wire up Codex

For one repository, Codex reads `AGENTS.md` from the project root. The
installer copies it plus the shared skill:

```bash
sh <noli-repo>/integrations/codex/install.sh <your-project>
```

This creates:

```text
<your-project>/AGENTS.md                                             # Noli usage rules
<your-project>/.agents/skills/noli-project-knowledge/SKILL.md        # operation reference
<your-project>/.agents/skills/noli-project-knowledge/noli-starter.yaml # bootstrap config
```

If `AGENTS.md` already exists the installer leaves it alone and tells you to
merge manually. Then start Codex in the project and give it a task; the
instructions steer it to run `noli retrieve` before writing code.

To enable the first-run Noli decision in every repository for the current
user, install the integration globally:

```bash
sh <noli-repo>/integrations/codex/install.sh --global
```

This installs the skill at
`$HOME/.agents/skills/noli-project-knowledge` and adds a managed Noli block to
the active global Codex guidance file under `${CODEX_HOME:-$HOME/.codex}`. An
existing non-empty `AGENTS.override.md` is active; otherwise the installer
uses `AGENTS.md`. Existing guidance is preserved, and repeated installation
does not duplicate the block. Override the skill root for testing or custom
layouts with `NOLI_CODEX_SKILLS_DIR`.

In a repository with no `noli.yaml`, `knowledge/`, or opt-out sentinel, Codex
asks once whether to initialize Noli. A yes creates and validates the local
knowledge base; a no records `.noli/disabled`, so later sessions do not ask
again. Repositories already enabled or opted out never ask.

## 6. Wire up Pi

Pi can discover a project-local extension that exposes five native read-only
tools: `noli_status`, `noli_search`, `noli_retrieve`, `noli_get`, and `noli_graph`.

```bash
sh <noli-repo>/integrations/pi/install.sh <your-project>
```

This installs `.pi/extensions/noli/index.ts`, its subprocess runner, and the
shared skill under `.agents/skills`. To enable the extension, skill, and
first-run decision in every repository for the current user:

```bash
sh <noli-repo>/integrations/pi/install.sh --global
```

Global mode installs the extension under `~/.pi/agent/extensions`, adds a
managed Noli block to `~/.pi/agent/AGENTS.md`, and reuses
`~/.agents/skills/noli-project-knowledge` with Codex. Override these roots
with `PI_CODING_AGENT_DIR` and `NOLI_AGENT_SKILLS_DIR` when needed. Existing
global guidance is preserved and repeat installation is idempotent.

Ensure `noli` is on `PATH`, or set an absolute executable path before
starting Pi:

```bash
export NOLI_BINARY_PATH="$HOME/.local/bin/noli"
cd <your-project>
pi
```

Pi derives the repository root from its working directory. Override it only
when necessary with an absolute `NOLI_REPOSITORY_ROOT`. Tool inputs cannot pick
the executable, execute a shell, or escape the repository. Smoke-test the
adapter against the installed Pi version with:

```bash
npm --prefix <noli-repo>/integrations/pi test
```

## 7. The developer stays in control (opt-in per repository)

Installing the integration does **not** force Noli onto a project. On the
first task in a repository the agent checks three states:

| State | Meaning | Agent behavior |
|---|---|---|
| `noli.yaml` or `knowledge/` exists | Noli enabled | Retrieves knowledge before coding |
| `.noli/disabled` exists | Developer opted out | Works normally, never mentions Noli |
| Legacy `okf.yaml` or `.okf/disabled` only | Migration fallback | Honors the legacy choice until a Noli namespace is added |
| Neither | Undecided | Asks you once: *"Initialize a Noli knowledge base? (yes/no)"* |

Answer **yes** and the agent bootstraps a knowledge base for you: it copies
the shipped `noli-starter.yaml` to `./noli.yaml` and
`noli-starter-concepts.yaml` to `.noli/concepts.yaml`, runs
`noli generate --config noli.yaml --apply` (which renders a valid starter
bundle with indexes and a log), and validates the result.

From then on `.noli/concepts.yaml` is the single source of truth. When you
ask for something new — say *"build a short URL generator with Python"* —
the agent first records the design there (entities like `Short Link`,
rules like `Slug Uniqueness`, components like `Shortener Service`,
workflows like `Resolve URL`, plus architecture decisions, all connected
with typed relationships), re-runs `generate --apply` so the Markdown
bundle and its knowledge graph are regenerated deterministically, and only
then writes code — retrieving from the knowledge it just created. When
implementation reveals new facts, it updates the concepts and re-applies,
so knowledge and code never drift apart.

Answer **no** and the agent writes `.noli/disabled` and behaves as if Noli
were not installed. Delete that file (or create `noli.yaml`) any time to
turn Noli on later.

The Pi adapter enforces the same choice mechanically: when
`.noli/disabled` exists, every tool call fails fast with the stable code
`NOLI_DISABLED` so the agent knows to proceed without it.

You can also decide manually up front:

```bash
# opt in without being asked:
cp <noli-repo>/integrations/shared/noli-starter.yaml noli.yaml   # edit project.name
noli generate --config noli.yaml --apply --format json

# opt out without being asked:
mkdir -p .noli && echo "developer opted out" > .noli/disabled
```

## 8. Using all three agents on the same repository

Claude Code, Codex, and Pi coexist cleanly because they share the same local
bundle while using their native integration surfaces:

```text
your-project/
├── CLAUDE.md                    # Claude Code instructions
├── AGENTS.md                    # Codex instructions
├── .claude/commands/noli-context.md
├── .agents/skills/noli-project-knowledge/SKILL.md
├── .pi/extensions/noli/{index.ts,runner.ts}
├── noli.yaml
└── knowledge/                   # shared source of truth for all three agents
```

All three agents call the same binary against the same bundle, so they see
identical, deterministic context — same query, byte-identical JSON. If two
agents try to regenerate the bundle or replace the same prepared-context
directory concurrently, a target-specific interprocess lock lets one writer
finish and makes the other fail safely so it can retry. Unique staging and
backup directories prevent cross-process cleanup collisions.

A good comparative experiment:

1. Ask Claude Code: *"Implement the CompleteTodo use case; cite the
   knowledge documents you relied on."*
2. Ask Codex the same sentence.
3. Compare which sources each cites — both should ground themselves on
   `rules/complete-task`, `concepts/todo-item`, `concepts/task-status`,
   `components/todo-service`, `components/todo-repository`, and
   `decisions/domain-validation-layer`.

## 9. Pre-baked contexts (optional, faster agent startup)

Instead of retrieving at question time, prepare context files ahead of an
agent session:

```bash
noli prepare-agent-context --root knowledge \
  --config noli-agent-queries.yaml \
  --output .noli/agent-context --format json
```

Point agents at `.noli/agent-context/<query-name>.md`; `manifest.json`
records the bundle checksum, per-file SHA-256 checksums, and the sources
behind every context.

## 10. Keeping knowledge honest

After any knowledge edit (by you or by an agent):

```bash
noli validate --root knowledge --mode project --config noli.yaml --format json
```

Exit code 0 means valid; exit code 4 returns the full error report in
`data.errors`. Both `CLAUDE.md` and `AGENTS.md` instruct the agents to run
this themselves after editing documents.

## Troubleshooting

| Symptom | Fix |
|---|---|
| `command not found: noli` | Re-run `scripts/install-local.sh`; ensure the install dir is on `PATH`. |
| `KNOWLEDGE_NOT_FOUND` (exit 3) | `--root` must point at the knowledge directory, not the project root. |
| `PARSE_ERROR` on your own docs | Every document needs `---` YAML frontmatter with a `type`. |
| Agent reads knowledge files directly | Make sure `CLAUDE.md` / `AGENTS.md` landed in the project root the agent was started in. |
| Empty retrieval results | Query words must appear in titles, descriptions, tags, or bodies; try `noli search` to debug scoring. |
| `GENERATION_FAILED` says the target is locked | Another agent is writing the same bundle or prepared-context target; wait for it to finish, then retry. |
| A lock remains busy after a crash on non-Linux | Confirm no writer is active, then remove the stale `.d` lock directory named in the error. Linux locks are released by the kernel. |
