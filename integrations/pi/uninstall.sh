#!/bin/sh
# Remove Noli's user-global Pi integration without deleting project knowledge
# or unrelated/modified Pi files.
set -eu

SOURCE="$(cd "$(dirname "$0")" && pwd)"
. "$SOURCE/../shared/uninstall-managed.sh"

usage() {
    echo "usage: uninstall.sh --global" >&2
}

uninstall_global() {
    : "${HOME:?HOME must be set for --global}"
    PI_AGENT_DIRECTORY="${PI_CODING_AGENT_DIR:-$HOME/.pi/agent}"
    PI_SHARED_SKILLS="${NOLI_PI_SKILLS_DIR:-${NOLI_AGENT_SKILLS_DIR:-$HOME/.agents/skills}}"
    CODEX_DIRECTORY="${CODEX_HOME:-$HOME/.codex}"
    CODEX_SKILLS="${NOLI_CODEX_SKILLS_DIR:-$HOME/.agents/skills}"

    noli_remove_managed_guidance \
        "$PI_AGENT_DIRECTORY/AGENTS.md" \
        "$SOURCE/../shared/GLOBAL_GUIDANCE.md" \
        "global Noli Pi guidance"
    noli_remove_managed_file \
        "$PI_AGENT_DIRECTORY/extensions/noli/index.ts" \
        "$SOURCE/extension.ts" \
        "global Noli Pi extension"
    noli_remove_managed_file \
        "$PI_AGENT_DIRECTORY/extensions/noli/runner.ts" \
        "$SOURCE/runner.ts" \
        "global Noli Pi runner"
    noli_remove_managed_file \
        "$PI_AGENT_DIRECTORY/extensions/noli/noli-starter.yaml" \
        "$SOURCE/../shared/noli-starter.yaml" \
        "global Noli Pi starter config"
    noli_remove_managed_file \
        "$PI_AGENT_DIRECTORY/extensions/noli/noli-starter-concepts.yaml" \
        "$SOURCE/../shared/noli-starter-concepts.yaml" \
        "global Noli Pi starter concepts"
    rmdir "$PI_AGENT_DIRECTORY/extensions/noli" 2>/dev/null || true
    rmdir "$PI_AGENT_DIRECTORY/extensions" 2>/dev/null || true

    if [ "$PI_SHARED_SKILLS" = "$CODEX_SKILLS" ] && noli_codex_integration_present "$CODEX_DIRECTORY"; then
        echo "retained shared Noli skill because the Codex integration is still installed"
    else
        noli_remove_managed_skill "$PI_SHARED_SKILLS" "$SOURCE/../shared"
    fi

    echo "uninstalled global Noli Pi integration"
}

case "${1:-}" in
--global)
    if [ "$#" -ne 1 ]; then
        usage
        exit 2
    fi
    uninstall_global
    ;;
-h|--help)
    usage
    ;;
"")
    usage
    exit 2
    ;;
-*)
    echo "unknown option: $1" >&2
    usage
    exit 2
    ;;
*)
    echo "only user-global uninstall is supported; project files are preserved" >&2
    usage
    exit 2
    ;;
esac
