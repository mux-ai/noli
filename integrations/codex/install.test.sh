#!/bin/sh
set -eu

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
INSTALLER="$ROOT/integrations/codex/install.sh"
TEST_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/noli-codex-installer.XXXXXX")
trap 'rm -rf "$TEST_ROOT"' EXIT HUP INT TERM

PROJECT="$TEST_ROOT/project"
mkdir -p "$PROJECT"
sh "$INSTALLER" "$PROJECT" >/dev/null
cmp "$ROOT/integrations/codex/AGENTS.md" "$PROJECT/AGENTS.md"
cmp "$ROOT/integrations/shared/SKILL.md" "$PROJECT/.agents/skills/noli-project-knowledge/SKILL.md"

TEST_HOME="$TEST_ROOT/home"
TEST_CODEX_HOME="$TEST_ROOT/codex-home"
TEST_SKILLS="$TEST_ROOT/user-skills"
mkdir -p "$TEST_HOME" "$TEST_CODEX_HOME"
printf '# Existing user guidance\n' >"$TEST_CODEX_HOME/AGENTS.md"

HOME="$TEST_HOME" CODEX_HOME="$TEST_CODEX_HOME" NOLI_CODEX_SKILLS_DIR="$TEST_SKILLS" \
    sh "$INSTALLER" --global >/dev/null
cmp "$ROOT/integrations/shared/SKILL.md" "$TEST_SKILLS/noli-project-knowledge/SKILL.md"
grep -qF '# Existing user guidance' "$TEST_CODEX_HOME/AGENTS.md"
test "$(grep -cF '<!-- noli-global-guidance:start -->' "$TEST_CODEX_HOME/AGENTS.md")" -eq 1
test "$(grep -cF '<!-- noli-global-guidance:end -->' "$TEST_CODEX_HOME/AGENTS.md")" -eq 1

HOME="$TEST_HOME" CODEX_HOME="$TEST_CODEX_HOME" NOLI_CODEX_SKILLS_DIR="$TEST_SKILLS" \
    sh "$INSTALLER" --global >/dev/null
test "$(grep -cF '<!-- noli-global-guidance:start -->' "$TEST_CODEX_HOME/AGENTS.md")" -eq 1

OVERRIDE_CODEX_HOME="$TEST_ROOT/codex-override-home"
mkdir -p "$OVERRIDE_CODEX_HOME"
printf '# Ignored base guidance\n' >"$OVERRIDE_CODEX_HOME/AGENTS.md"
printf '# Active override guidance\n' >"$OVERRIDE_CODEX_HOME/AGENTS.override.md"
HOME="$TEST_HOME" CODEX_HOME="$OVERRIDE_CODEX_HOME" NOLI_CODEX_SKILLS_DIR="$TEST_SKILLS" \
    sh "$INSTALLER" --global >/dev/null
grep -qF '<!-- noli-global-guidance:start -->' "$OVERRIDE_CODEX_HOME/AGENTS.override.md"
if grep -qF '<!-- noli-global-guidance:start -->' "$OVERRIDE_CODEX_HOME/AGENTS.md"; then
    echo "global guidance was added to the inactive AGENTS.md" >&2
    exit 1
fi

BROKEN_CODEX_HOME="$TEST_ROOT/codex-broken-home"
mkdir -p "$BROKEN_CODEX_HOME"
printf '<!-- noli-global-guidance:start -->\n' >"$BROKEN_CODEX_HOME/AGENTS.md"
if HOME="$TEST_HOME" CODEX_HOME="$BROKEN_CODEX_HOME" NOLI_CODEX_SKILLS_DIR="$TEST_SKILLS" \
    sh "$INSTALLER" --global >/dev/null 2>&1; then
    echo "installer accepted incomplete guidance markers" >&2
    exit 1
fi
