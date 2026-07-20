#!/bin/sh
set -eu

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
INSTALLER="$ROOT/integrations/claude/install.sh"
TEST_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/noli-claude-installer.XXXXXX")
trap 'rm -rf "$TEST_ROOT"' EXIT HUP INT TERM

PROJECT="$TEST_ROOT/project"
mkdir -p "$PROJECT"
sh "$INSTALLER" "$PROJECT" >/dev/null
cmp "$ROOT/integrations/claude/CLAUDE.md" "$PROJECT/CLAUDE.md"
cmp "$ROOT/integrations/claude/commands/noli-context.md" "$PROJECT/.claude/commands/noli-context.md"
cmp "$ROOT/integrations/shared/SKILL.md" "$PROJECT/.claude/skills/noli-project-knowledge/SKILL.md"

TEST_HOME="$TEST_ROOT/home"
TEST_CLAUDE_HOME="$TEST_ROOT/claude-home"
mkdir -p "$TEST_HOME" "$TEST_CLAUDE_HOME"
printf '# Existing Claude guidance\n' >"$TEST_CLAUDE_HOME/CLAUDE.md"

HOME="$TEST_HOME" CLAUDE_CONFIG_DIR="$TEST_CLAUDE_HOME" sh "$INSTALLER" --global >/dev/null
cmp "$ROOT/integrations/shared/SKILL.md" "$TEST_CLAUDE_HOME/skills/noli-project-knowledge/SKILL.md"
cmp "$ROOT/integrations/claude/commands/noli-context.md" "$TEST_CLAUDE_HOME/commands/noli-context.md"
grep -qF '# Existing Claude guidance' "$TEST_CLAUDE_HOME/CLAUDE.md"
test "$(grep -cF '<!-- noli-global-guidance:start -->' "$TEST_CLAUDE_HOME/CLAUDE.md")" -eq 1
test "$(grep -cF '<!-- noli-global-guidance:end -->' "$TEST_CLAUDE_HOME/CLAUDE.md")" -eq 1

HOME="$TEST_HOME" CLAUDE_CONFIG_DIR="$TEST_CLAUDE_HOME" sh "$INSTALLER" --global >/dev/null
test "$(grep -cF '<!-- noli-global-guidance:start -->' "$TEST_CLAUDE_HOME/CLAUDE.md")" -eq 1

BROKEN_CLAUDE_HOME="$TEST_ROOT/claude-broken-home"
mkdir -p "$BROKEN_CLAUDE_HOME"
printf '<!-- noli-global-guidance:start -->\n' >"$BROKEN_CLAUDE_HOME/CLAUDE.md"
if HOME="$TEST_HOME" CLAUDE_CONFIG_DIR="$BROKEN_CLAUDE_HOME" \
    sh "$INSTALLER" --global >/dev/null 2>&1; then
    echo "installer accepted incomplete guidance markers" >&2
    exit 1
fi
