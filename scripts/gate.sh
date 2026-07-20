#!/usr/bin/env bash
# Phase gate runner for Noli (PLANS.md section 8).
#
# Usage:
#   scripts/gate.sh          # run every gate whose packages exist
#   scripts/gate.sh 1        # run a single phase gate (cumulative rules apply)
#
# A gate is four layers:
#   1. automated commands (tests, vet, build) — must exit 0;
#   2. dependency-direction rules (PLANS.md section 3) — enforced via go list;
#   3. frozen-contract lock — PROTOCOL.md and fixtures must match
#      docs/fixtures.sha256; intentional changes regenerate the lock with
#      reviewer sign-off;
#   4. reviewer sign-off items — printed as reminders, not automatable.
set -u
cd "$(dirname "$0")/.."
export GOCACHE="${GOCACHE:-/tmp/noli-gocache}"

PHASE="${1:-all}"
PASS=0
FAIL=0
SKIP=0
LOG="$(mktemp)"
trap 'rm -f "$LOG"' EXIT

check() {
    local name="$1"
    shift
    if "$@" >"$LOG" 2>&1; then
        printf 'PASS  %s\n' "$name"
        PASS=$((PASS + 1))
    else
        printf 'FAIL  %s\n' "$name"
        head -20 "$LOG" | sed 's/^/      /'
        FAIL=$((FAIL + 1))
    fi
}

skip() {
    printf 'SKIP  %s (not implemented yet)\n' "$1"
    SKIP=$((SKIP + 1))
}

note() {
    printf 'NOTE  %s\n' "$1"
}

# dep_rule <package path> <allowed module-internal import regex>
# Fails if the package (or anything it pulls in) imports a module-internal
# package outside the allowed set. Enforces the frozen dependency direction.
dep_rule() {
    local pkg="$1"
    local allowed="$2"
    local bad
    bad=$(go list -deps "./$pkg" 2>/dev/null | grep '^noli/' | grep -vE "$allowed" || true)
    if [ -n "$bad" ]; then
        echo "forbidden module-internal imports reached from $pkg:"
        echo "$bad"
        return 1
    fi
}

fixtures_json_valid() {
    local file
    for file in pkg/protocol/testdata/fixtures/*/*.json; do
        python3 -m json.tool "$file" >/dev/null || return 1
    done
}

repository_graph_smoke() {
    local output
    output=$(./bin/noli retrieve --root knowledge \
        --query "Coordinate three coding agents safely during knowledge generation and prepared context writes" \
        --search-limit 6 --max-hops 2 --max-documents 12 \
        --max-characters 18000 --direction both --format json) || return 1
    NOLI_GATE_RETRIEVAL="$output" python3 -c '
import json, os
payload = json.loads(os.environ["NOLI_GATE_RETRIEVAL"])
sources = {item["id"] for item in payload["data"]["sources"]}
required = {
    "workflows/three-agent-collaboration",
    "rules/single-writer-per-target",
    "integrations/pi-coding-agent",
}
assert required <= sources, (required - sources)
'
}

repository_generation_clean() {
    local output
    output=$(./bin/noli generate --config noli.yaml --dry-run --format json) || {
        rm -rf .noli/preview
        return 1
    }
    if ! NOLI_GATE_GENERATE="$output" python3 -c '
import json, os
data = json.loads(os.environ["NOLI_GATE_GENERATE"])["data"]
assert not data["added"], data["added"]
assert not data["changed"], data["changed"]
assert not data["removed"], data["removed"]
'; then
        rm -rf .noli/preview
        return 1
    fi
    rm -rf .noli/preview
}

noli_namespace_clean() {
    test -f noli.yaml || return 1
    test -f noli-agent-queries.yaml || return 1
    test -f .noli/concepts.yaml || return 1
    test -f integrations/claude/commands/noli-context.md || return 1
    test -f integrations/claude/install.sh || return 1
    test -f integrations/claude/uninstall.sh || return 1
    test -f integrations/codex/GLOBAL_AGENTS.md || return 1
    test -f integrations/codex/uninstall.sh || return 1
    test -f integrations/pi/uninstall.sh || return 1
    test -f integrations/shared/GLOBAL_GUIDANCE.md || return 1
    test -f integrations/shared/install-managed-guidance.sh || return 1
    test -f integrations/shared/uninstall-managed.sh || return 1
    test -f integrations/shared/noli-starter.yaml || return 1
    test -f integrations/shared/noli-starter-concepts.yaml || return 1
    test -f scripts/uninstall-global.sh || return 1
    test -f examples/todo-app/noli.yaml || return 1
    test ! -e okf.yaml || return 1
    test ! -e okf-agent-queries.yaml || return 1
    test ! -e .okf || return 1
}

legacy_cli_alias_smoke() {
    local primary legacy
    primary=$(./bin/noli status --root examples/todo-app/knowledge --format json) || return 1
    legacy=$(./bin/okf status --root examples/todo-app/knowledge --format json) || return 1
    test "$primary" = "$legacy"
}

gate0() {
    echo "== Phase 0 gate: baseline and frozen contracts =="
    check "noligen still builds" go build -buildvcs=false -o "$GOCACHE/noligen-gate" ./cmd/noligen
    check "legacy okfgen alias still builds" go build -buildvcs=false -o "$GOCACHE/okfgen-gate" ./cmd/okfgen
    check "existing internal packages stay green" go test ./internal/...
    check "all fixtures parse as JSON" fixtures_json_valid
    check "frozen contract lock (docs/fixtures.sha256)" sha256sum --quiet -c docs/fixtures.sha256
    note "reviewer sign-off required once for docs/PROTOCOL.md + fixtures (granted 2026-07-20)"
}

gate1() {
    echo "== Phase 1 gate: leaf packages =="
    check "go test pkg/graph pkg/search" go test ./pkg/graph ./pkg/search
    check "go vet pkg/graph pkg/search" go vet ./pkg/graph ./pkg/search
    check "dep rule: pkg/graph imports nothing module-internal" dep_rule pkg/graph '^noli/pkg/graph$'
    check "dep rule: pkg/search imports nothing module-internal" dep_rule pkg/search '^noli/pkg/search$'
}

gate2() {
    if [ ! -d pkg/okf ]; then skip "Phase 2 gate (pkg/okf)"; return; fi
    echo "== Phase 2 gate: public OKF model, parser, store =="
    check "go test pkg/okf" go test ./pkg/okf
    check "go vet pkg/okf" go vet ./pkg/okf
    check "dep rule: pkg/okf uses only graph/search/retrieval and their lock helper" \
        dep_rule pkg/okf '^noli/pkg/(okf|graph|search|retrieval|internal/targetlock)$'
}

gate3() {
    if [ ! -d pkg/retrieval ]; then skip "Phase 3 gate (pkg/retrieval)"; return; fi
    echo "== Phase 3 gate: retrieval and read APIs =="
    check "go test pkg/retrieval pkg/okf" go test ./pkg/retrieval ./pkg/okf
    check "go vet pkg/retrieval pkg/okf" go vet ./pkg/retrieval ./pkg/okf
    check "dep rule: pkg/retrieval never imports pkg/okf" \
        dep_rule pkg/retrieval '^noli/pkg/(retrieval|graph|search|internal/targetlock)$'
}

gate4() {
    if [ ! -d cmd/noli ]; then skip "Phase 4 gate (protocol and read-only CLI)"; return; fi
    echo "== Phase 4 gate: stable protocol and read-only CLI =="
    check "go test protocol/cli/cmd" go test ./pkg/protocol ./internal/cli ./cmd/noli ./cmd/okf
    check "noli binary builds" go build -buildvcs=false -o ./bin/noli ./cmd/noli
    check "legacy okf alias builds" go build -buildvcs=false -o ./bin/okf ./cmd/okf
    check "legacy okf alias matches noli output" legacy_cli_alias_smoke
    check "frozen contract lock (docs/fixtures.sha256)" sha256sum --quiet -c docs/fixtures.sha256
    note "CLI tests must cover: every success/error shape, stdout/stderr split, exits 0/2/3/4/6/7, double-run determinism"
}

gate5() {
    if [ ! -f pkg/generator/config.go ]; then
        skip "Phase 5 gate (strict config and project validation)"
        return
    fi
    echo "== Phase 5 gate: strict config and project validation =="
    check "go test pkg/generator" go test ./pkg/generator
    check "go vet pkg/generator" go vet ./pkg/generator
    check "dep rule: pkg/generator uses only public pkg packages" \
        dep_rule pkg/generator '^noli/pkg/(generator|okf|graph|search|retrieval|internal/targetlock)$'
    check "go test internal/cli (project validation)" go test ./internal/cli
    note "Todo config must validate without invented confidence/citation values"
}

gate6() {
    if [ ! -f pkg/generator/generator.go ]; then skip "Phase 6 gate (deterministic generation)"; return; fi
    echo "== Phase 6 gate: deterministic generation =="
    check "go test pkg/generator" go test ./pkg/generator
    check "go vet pkg/generator" go vet ./pkg/generator
    check "go test internal/cli (generate command)" go test -run 'TestGenerate' ./internal/cli
    note "tests prove: dry-run leaves knowledge byte-identical; failed apply rolls back; identical inputs render identical Markdown"
}

gate7() {
    if [ ! -f pkg/retrieval/prepare.go ]; then skip "Phase 7 gate (prepared agent contexts)"; return; fi
    echo "== Phase 7 gate: prepared agent contexts =="
    check "go test pkg/retrieval" go test ./pkg/retrieval
    check "dep rule: pkg/retrieval never imports pkg/okf" \
        dep_rule pkg/retrieval '^noli/pkg/(retrieval|graph|search|internal/targetlock)$'
    check "go test internal/cli (prepare command)" go test -run 'TestPrepare' ./internal/cli
    note "tests prove: manifest checksums reproduce; invalid query/output paths cannot escape the project"
}

gate8() {
    if [ ! -d integrations/pi ]; then skip "Phase 8 gate (Todo example and integrations)"; return; fi
    echo "== Phase 8 gate: Todo example and thin integrations =="
    check "pi integration tests" npm --prefix integrations/pi test
    check "install-local.sh syntax" sh -n scripts/install-local.sh
    check "uninstall-global.sh syntax" sh -n scripts/uninstall-global.sh
    check "managed guidance helper syntax" sh -n integrations/shared/install-managed-guidance.sh
    check "managed uninstall helper syntax" sh -n integrations/shared/uninstall-managed.sh
    check "claude install.sh syntax" sh -n integrations/claude/install.sh
    check "claude uninstall.sh syntax" sh -n integrations/claude/uninstall.sh
    check "claude installer behavior" sh integrations/claude/install.test.sh
    check "codex install.sh syntax" sh -n integrations/codex/install.sh
    check "codex uninstall.sh syntax" sh -n integrations/codex/uninstall.sh
    check "codex installer behavior" sh integrations/codex/install.test.sh
    check "pi install.sh syntax" sh -n integrations/pi/install.sh
    check "pi uninstall.sh syntax" sh -n integrations/pi/uninstall.sh
    check "pi installer behavior" sh integrations/pi/install.test.sh
    check "global uninstaller behavior" sh scripts/uninstall-global.test.sh
    check "Noli primary namespace files" noli_namespace_clean
    if [ -f noli.yaml ] && [ -d knowledge ]; then
        check "repository knowledge: OKF v0.1 standard validation" \
            ./bin/noli validate --root knowledge --mode standard --format json
        check "repository knowledge: project validation" \
            ./bin/noli validate --root knowledge --mode project --config noli.yaml --format json
        check "repository knowledge: three-agent graph smoke" repository_graph_smoke
        check "repository knowledge: deterministic source matches bundle" repository_generation_clean
    fi
}

gate9() {
    if [ ! -d cmd/noli ]; then skip "Phase 9 gate (final acceptance)"; return; fi
    echo "== Phase 9 gate: full acceptance (PLANS.md section 12) =="
    check "gofmt clean" test -z "$(gofmt -l ./cmd ./pkg ./internal 2>/dev/null)"
    check "full go test" go test ./...
    check "full go test -race" go test -race ./...
    check "full go vet" go vet ./...
    note "run every JSON command from PLANS.md section 12 against the Todo example; capture stdout/stderr separately"
}

case "$PHASE" in
all)
    for n in 0 1 2 3 4 5 6 7 8 9; do "gate$n"; done
    ;;
[0-9])
    "gate$PHASE"
    ;;
*)
    echo "usage: scripts/gate.sh [0-9|all]" >&2
    exit 2
    ;;
esac

echo
echo "gate result: $PASS passed, $FAIL failed, $SKIP skipped"
[ "$FAIL" -eq 0 ]
