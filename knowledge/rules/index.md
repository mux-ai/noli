# Business Rule

* [Bounded Deterministic Retrieval](/rules/bounded-deterministic-retrieval.md) - Retrieval must be reproducible, size-bounded, UTF-8 safe, and traceable to selected concepts.
* [Codex Global First-Run Choice](/rules/codex-global-first-run-choice.md) - User-global Codex guidance asks once before enabling Noli in a repository that has no recorded Noli state.
* [Frozen Protocol Compatibility](/rules/frozen-protocol-compatibility.md) - Agent-visible JSON shapes, error codes, and exit mappings change only through an explicit contract update.
* [Generated Knowledge Source of Truth](/rules/generated-knowledge-source-of-truth.md) - When generation is configured, agents edit structured concepts rather than generated Markdown.
* [Noli Namespace Compatibility](/rules/noli-namespace-compatibility.md) - Noli owns the primary product namespace while legacy OKF product identifiers remain bounded deprecated aliases.
* [OKF v0.1 Conformance](/rules/okf-v0-1-conformance.md) - Standard validation accepts the permissive Open Knowledge Format v0.1 consumption model.
* [Repository Path Containment](/rules/repository-path-containment.md) - Local tools must not read, write, or execute through paths that escape the configured repository.
* [Single Writer Per Target](/rules/single-writer-per-target.md) - At most one process may replace a project bundle or prepared-context output at a time.
