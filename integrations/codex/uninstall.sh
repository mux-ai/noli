#!/bin/sh
# Remove Noli's user-global Codex integration without deleting project
# knowledge or unrelated/modified Codex files.
set -eu

SOURCE="$(cd "$(dirname "$0")" && pwd)"
. "$SOURCE/../shared/uninstall-managed.sh"

usage() {
    echo "usage: uninstall.sh --global" >&2
}

uninstall_global() {
    : "${HOME:?HOME must be set for --global}"
    CODEX_DIRECTORY="${CODEX_HOME:-$HOME/.codex}"
    CODEX_SKILLS="${NOLI_CODEX_SKILLS_DIR:-$HOME/.agents/skills}"
    PI_AGENT_DIRECTORY="${PI_CODING_AGENT_DIR:-$HOME/.pi/agent}"
    PI_SHARED_SKILLS="${NOLI_PI_SKILLS_DIR:-${NOLI_AGENT_SKILLS_DIR:-$HOME/.agents/skills}}"

    noli_remove_managed_guidance \
        "$CODEX_DIRECTORY/AGENTS.override.md" \
        "$SOURCE/GLOBAL_AGENTS.md" \
        "global Noli Codex override guidance"
    noli_remove_managed_guidance \
        "$CODEX_DIRECTORY/AGENTS.md" \
        "$SOURCE/GLOBAL_AGENTS.md" \
        "global Noli Codex guidance"

    if [ "$CODEX_SKILLS" = "$PI_SHARED_SKILLS" ] && noli_pi_integration_present "$PI_AGENT_DIRECTORY"; then
        echo "retained shared Noli skill because the Pi integration is still installed"
    else
        noli_remove_managed_skill "$CODEX_SKILLS" "$SOURCE/../shared"
    fi

    echo "uninstalled global Noli Codex integration"
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
