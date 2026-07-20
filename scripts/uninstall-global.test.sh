#!/bin/sh
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
UNINSTALLER="$ROOT/scripts/uninstall-global.sh"
TEST_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/noli-global-uninstaller.XXXXXX")
trap 'rm -rf "$TEST_ROOT"' EXIT HUP INT TERM

TEST_HOME="$TEST_ROOT/home"
CLAUDE_HOME="$TEST_ROOT/claude"
CODEX_HOME_TEST="$TEST_ROOT/codex"
PI_HOME="$TEST_ROOT/pi"
SHARED_SKILLS="$TEST_ROOT/agent-skills"
mkdir -p "$TEST_HOME" "$CLAUDE_HOME" "$CODEX_HOME_TEST" "$PI_HOME"
printf '# Existing Claude guidance\n' >"$CLAUDE_HOME/CLAUDE.md"
printf '# Existing Codex guidance\n' >"$CODEX_HOME_TEST/AGENTS.md"
printf '# Existing Pi guidance\n' >"$PI_HOME/AGENTS.md"

HOME="$TEST_HOME" CLAUDE_CONFIG_DIR="$CLAUDE_HOME" \
    sh "$ROOT/integrations/claude/install.sh" --global >/dev/null
HOME="$TEST_HOME" CODEX_HOME="$CODEX_HOME_TEST" NOLI_CODEX_SKILLS_DIR="$SHARED_SKILLS" \
    sh "$ROOT/integrations/codex/install.sh" --global >/dev/null
HOME="$TEST_HOME" PI_CODING_AGENT_DIR="$PI_HOME" NOLI_AGENT_SKILLS_DIR="$SHARED_SKILLS" \
    sh "$ROOT/integrations/pi/install.sh" --global >/dev/null

test -f "$CLAUDE_HOME/skills/noli-project-knowledge/SKILL.md"
test -f "$CODEX_HOME_TEST/AGENTS.md"
test -f "$PI_HOME/extensions/noli/index.ts"
test -f "$SHARED_SKILLS/noli-project-knowledge/SKILL.md"

HOME="$TEST_HOME" CLAUDE_CONFIG_DIR="$CLAUDE_HOME" CODEX_HOME="$CODEX_HOME_TEST" \
    NOLI_CODEX_SKILLS_DIR="$SHARED_SKILLS" PI_CODING_AGENT_DIR="$PI_HOME" \
    NOLI_AGENT_SKILLS_DIR="$SHARED_SKILLS" sh "$UNINSTALLER" >/dev/null

grep -qF '# Existing Claude guidance' "$CLAUDE_HOME/CLAUDE.md"
grep -qF '# Existing Codex guidance' "$CODEX_HOME_TEST/AGENTS.md"
grep -qF '# Existing Pi guidance' "$PI_HOME/AGENTS.md"
if grep -qF '<!-- noli-global-guidance:' "$CLAUDE_HOME/CLAUDE.md" \
    "$CODEX_HOME_TEST/AGENTS.md" "$PI_HOME/AGENTS.md"; then
    echo "global uninstaller left managed guidance markers" >&2
    exit 1
fi
test ! -e "$CLAUDE_HOME/commands/noli-context.md"
test ! -e "$CLAUDE_HOME/skills/noli-project-knowledge"
test ! -e "$PI_HOME/extensions/noli"
test ! -e "$SHARED_SKILLS/noli-project-knowledge"

HOME="$TEST_HOME" CLAUDE_CONFIG_DIR="$CLAUDE_HOME" CODEX_HOME="$CODEX_HOME_TEST" \
    NOLI_CODEX_SKILLS_DIR="$SHARED_SKILLS" PI_CODING_AGENT_DIR="$PI_HOME" \
    NOLI_AGENT_SKILLS_DIR="$SHARED_SKILLS" sh "$UNINSTALLER" >/dev/null

RETAIN_HOME="$TEST_ROOT/retain-home"
RETAIN_CODEX="$TEST_ROOT/retain-codex"
RETAIN_PI="$TEST_ROOT/retain-pi"
RETAIN_SKILLS="$TEST_ROOT/retain-skills"
mkdir -p "$RETAIN_HOME" "$RETAIN_CODEX" "$RETAIN_PI"
HOME="$RETAIN_HOME" CODEX_HOME="$RETAIN_CODEX" NOLI_CODEX_SKILLS_DIR="$RETAIN_SKILLS" \
    sh "$ROOT/integrations/codex/install.sh" --global >/dev/null
HOME="$RETAIN_HOME" PI_CODING_AGENT_DIR="$RETAIN_PI" NOLI_AGENT_SKILLS_DIR="$RETAIN_SKILLS" \
    sh "$ROOT/integrations/pi/install.sh" --global >/dev/null
HOME="$RETAIN_HOME" CODEX_HOME="$RETAIN_CODEX" NOLI_CODEX_SKILLS_DIR="$RETAIN_SKILLS" \
    PI_CODING_AGENT_DIR="$RETAIN_PI" NOLI_AGENT_SKILLS_DIR="$RETAIN_SKILLS" \
    sh "$ROOT/integrations/codex/uninstall.sh" --global >/dev/null
test -f "$RETAIN_SKILLS/noli-project-knowledge/SKILL.md"
HOME="$RETAIN_HOME" CODEX_HOME="$RETAIN_CODEX" NOLI_CODEX_SKILLS_DIR="$RETAIN_SKILLS" \
    PI_CODING_AGENT_DIR="$RETAIN_PI" NOLI_AGENT_SKILLS_DIR="$RETAIN_SKILLS" \
    sh "$ROOT/integrations/pi/uninstall.sh" --global >/dev/null
test ! -e "$RETAIN_SKILLS/noli-project-knowledge"

MODIFIED_HOME="$TEST_ROOT/modified-home"
MODIFIED_PI="$TEST_ROOT/modified-pi"
MODIFIED_SKILLS="$TEST_ROOT/modified-skills"
mkdir -p "$MODIFIED_HOME" "$MODIFIED_PI"
HOME="$MODIFIED_HOME" PI_CODING_AGENT_DIR="$MODIFIED_PI" NOLI_AGENT_SKILLS_DIR="$MODIFIED_SKILLS" \
    sh "$ROOT/integrations/pi/install.sh" --global >/dev/null
printf '\n// user modification\n' >>"$MODIFIED_PI/extensions/noli/index.ts"
HOME="$MODIFIED_HOME" PI_CODING_AGENT_DIR="$MODIFIED_PI" NOLI_AGENT_SKILLS_DIR="$MODIFIED_SKILLS" \
    sh "$ROOT/integrations/pi/uninstall.sh" --global >/dev/null 2>&1
test -f "$MODIFIED_PI/extensions/noli/index.ts"
grep -qF '// user modification' "$MODIFIED_PI/extensions/noli/index.ts"
test ! -e "$MODIFIED_PI/extensions/noli/runner.ts"

BROKEN_HOME="$TEST_ROOT/broken-home"
BROKEN_CLAUDE="$TEST_ROOT/broken-claude"
mkdir -p "$BROKEN_HOME" "$BROKEN_CLAUDE"
printf '<!-- noli-global-guidance:start -->\n' >"$BROKEN_CLAUDE/CLAUDE.md"
if HOME="$BROKEN_HOME" CLAUDE_CONFIG_DIR="$BROKEN_CLAUDE" \
    sh "$ROOT/integrations/claude/uninstall.sh" --global >/dev/null 2>&1; then
    echo "global uninstaller accepted incomplete guidance markers" >&2
    exit 1
fi
grep -qxF '<!-- noli-global-guidance:start -->' "$BROKEN_CLAUDE/CLAUDE.md"
