---
description: Retrieve bounded project knowledge with Noli
---

Retrieve Noli project knowledge for this task: $ARGUMENTS

1. Locate the knowledge root: use `knowledge/` in the repository root, or
   the `knowledge.root` value from `noli.yaml` when present.
2. Run:

```bash
noli retrieve --root knowledge --query "$ARGUMENTS" \
  --search-limit 5 --max-hops 1 --max-documents 8 --max-characters 14000 \
  --direction both --format json
```

3. Read `data.context` and summarize the rules, entities, and components
   relevant to the task. Reference documents by the IDs from
   `data.sources`.
4. If the command fails, report `error.code` and `error.message` verbatim
   and stop; do not guess at knowledge content.
