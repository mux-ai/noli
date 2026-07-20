#!/bin/sh
set -eu

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
INSTALLER="$ROOT/integrations/pi/install.sh"
TEST_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/noli-pi-installer.XXXXXX")
trap 'rm -rf "$TEST_ROOT"' EXIT HUP INT TERM

PROJECT="$TEST_ROOT/project"
mkdir -p "$PROJECT"
sh "$INSTALLER" "$PROJECT" >/dev/null 2>&1
cmp "$ROOT/integrations/pi/extension.ts" "$PROJECT/.pi/extensions/noli/index.ts"
cmp "$ROOT/integrations/pi/runner.ts" "$PROJECT/.pi/extensions/noli/runner.ts"
cmp "$ROOT/integrations/shared/SKILL.md" "$PROJECT/.agents/skills/noli-project-knowledge/SKILL.md"

TEST_HOME="$TEST_ROOT/home"
TEST_PI_HOME="$TEST_ROOT/pi-agent-home"
TEST_SKILLS="$TEST_ROOT/agent-skills"
mkdir -p "$TEST_HOME" "$TEST_PI_HOME"
printf '# Existing Pi guidance\n' >"$TEST_PI_HOME/AGENTS.md"

HOME="$TEST_HOME" PI_CODING_AGENT_DIR="$TEST_PI_HOME" NOLI_AGENT_SKILLS_DIR="$TEST_SKILLS" \
    sh "$INSTALLER" --global >/dev/null
cmp "$ROOT/integrations/pi/extension.ts" "$TEST_PI_HOME/extensions/noli/index.ts"
cmp "$ROOT/integrations/pi/runner.ts" "$TEST_PI_HOME/extensions/noli/runner.ts"
cmp "$ROOT/integrations/shared/SKILL.md" "$TEST_SKILLS/noli-project-knowledge/SKILL.md"
grep -qF '# Existing Pi guidance' "$TEST_PI_HOME/AGENTS.md"
test "$(grep -cF '<!-- noli-global-guidance:start -->' "$TEST_PI_HOME/AGENTS.md")" -eq 1
test "$(grep -cF '<!-- noli-global-guidance:end -->' "$TEST_PI_HOME/AGENTS.md")" -eq 1

HOME="$TEST_HOME" PI_CODING_AGENT_DIR="$TEST_PI_HOME" NOLI_AGENT_SKILLS_DIR="$TEST_SKILLS" \
    sh "$INSTALLER" --global >/dev/null
test "$(grep -cF '<!-- noli-global-guidance:start -->' "$TEST_PI_HOME/AGENTS.md")" -eq 1

BROKEN_PI_HOME="$TEST_ROOT/pi-broken-home"
mkdir -p "$BROKEN_PI_HOME"
printf '<!-- noli-global-guidance:start -->\n' >"$BROKEN_PI_HOME/AGENTS.md"
if HOME="$TEST_HOME" PI_CODING_AGENT_DIR="$BROKEN_PI_HOME" NOLI_AGENT_SKILLS_DIR="$TEST_SKILLS" \
    sh "$INSTALLER" --global >/dev/null 2>&1; then
    echo "installer accepted incomplete guidance markers" >&2
    exit 1
fi
