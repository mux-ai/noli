# Application Component

* [Bounded Retrieval Engine](/components/bounded-retrieval-engine.md) - Source-traceable context assembly from ranked search seeds and their graph neighborhoods.
* [Deterministic Knowledge Generator](/components/deterministic-generator.md) - The non-LLM renderer that converts structured concepts into a validated OKF bundle.
* [Interprocess Target Lock](/components/interprocess-target-lock.md) - A process-safe non-blocking lock preventing concurrent replacement of the same generated output.
* [Frozen JSON Protocol](/components/json-protocol.md) - Versioned deterministic success and error envelopes shared by all local agent integrations.
* [Keyword Search Engine](/components/keyword-search-engine.md) - Deterministic integer-scored keyword search over concept metadata and bodies.
* [Noli CLI](/components/noli-cli.md) - The local noli executable that powers Noli for agents and developers, with okf retained as a deprecated alias.
* [OKF Parser and Store](/components/parser-store.md) - The bounded parser and immutable in-memory representation of an OKF knowledge bundle.
* [Public Go SDK](/components/public-go-sdk.md) - The reusable Go facade for loading, querying, validating, and traversing OKF bundles.
* [Typed Graph Engine](/components/typed-graph-engine.md) - Cycle-safe directed graph traversal retaining distance, predecessor, relationship, and seed rank.
