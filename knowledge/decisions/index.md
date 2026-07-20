# Architecture Decision

* [Fail Fast on Writer Contention](/decisions/fail-fast-writer-contention.md) - A competing process receives a retryable generation failure instead of waiting indefinitely or racing filesystem replacement.
* [Local CLI Instead of a Service](/decisions/local-cli-over-service.md) - Coding agents access project knowledge through one local binary rather than MCP, HTTP, or a background daemon.
* [Markdown and YAML Knowledge Storage](/decisions/markdown-yaml-storage.md) - Store portable knowledge as Markdown concepts with YAML frontmatter following OKF v0.1.
* [Native Pi Tool Extension](/decisions/native-pi-extension.md) - Expose OKF read operations as native Pi tools through the installed Pi extension API.
* [Typed Relationship Interpretation](/decisions/typed-relationship-interpretation.md) - Interpret a small deterministic set of relationship phrases while keeping documents valid untyped OKF Markdown.
