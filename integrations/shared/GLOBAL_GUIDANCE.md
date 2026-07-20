<!-- noli-global-guidance:start -->
## Noli project-knowledge decision

At the start of the first substantive task in every repository, load and
follow the installed `noli-project-knowledge` skill and resolve the
repository's Noli state before implementation:

- If `noli.yaml` or `knowledge/` exists at the repository root, Noli is
  enabled. Use it without asking.
- If `.noli/disabled` exists, the developer opted out. Work normally without
  asking again.
- If neither exists, accept `okf.yaml` or `.okf/disabled` only as deprecated
  migration fallbacks. The Noli namespace wins when both exist.
- If no enabled or disabled state exists, ask exactly once:
  *"This project has Noli installed but no project knowledge base yet.
  Initialize one so I can retrieve grounded project knowledge? (yes/no)"*

On yes, follow the skill's bootstrap and authoring workflow. On no, create
`.noli/disabled` with `developer opted out` as its content, then work normally.
<!-- noli-global-guidance:end -->
