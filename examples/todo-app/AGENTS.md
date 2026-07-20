# Todo App — agent instructions

This repository demonstrates Noli. Project knowledge lives
in `knowledge/` as OKF Markdown documents. Do not read those files directly;
query them through the `noli` CLI so results stay bounded and deterministic.

## Getting context

Before implementing a feature, retrieve knowledge for your task:

```bash
noli retrieve --root knowledge \
  --query "<your task>" \
  --types "Business Rule,Domain Entity,Application Component,Architecture Decision" \
  --search-limit 5 --max-hops 1 --max-documents 8 --max-characters 14000 \
  --direction both --format json
```

The response is a JSON envelope; `data.context` holds Markdown you can read,
and `data.sources` traces every included document.

Other useful commands:

```bash
noli status   --root knowledge --format json
noli search   --root knowledge --query "task completion" --format json
noli get      --root knowledge --id rules/complete-task --format json
noli graph    --root knowledge --id rules/complete-task --direction both --format json
noli validate --root knowledge --mode project --config noli.yaml --format json
```

## Rules

- Success responses use exit code 0 and `"ok": true`; anything else is an
  error envelope with a stable `error.code`.
- Business rules retrieved from `rules/` are binding; follow them in code.
- After editing knowledge documents, run `noli validate --mode project
  --config noli.yaml` and fix every reported error.

All documents in `knowledge/` are demonstration data.
