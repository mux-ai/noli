#!/bin/sh
# Remove Noli's user-global Claude Code integration without deleting project
# knowledge or unrelated/modified Claude files.
set -eu

SOURCE="$(cd "$(dirname "$0")" && pwd)"
. "$SOURCE/../shared/uninstall-managed.sh"

usage() {
    echo "usage: uninstall.sh --global" >&2
}

uninstall_global() {
    : "${HOME:?HOME must be set for --global}"
    CLAUDE_DIRECTORY="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"

    noli_remove_managed_guidance \
        "$CLAUDE_DIRECTORY/CLAUDE.md" \
        "$SOURCE/../shared/GLOBAL_GUIDANCE.md" \
        "global Noli Claude Code guidance"
    noli_remove_managed_file \
        "$CLAUDE_DIRECTORY/commands/noli-context.md" \
        "$SOURCE/commands/noli-context.md" \
        "global /noli-context command"
    noli_remove_managed_skill "$CLAUDE_DIRECTORY/skills" "$SOURCE/../shared"
    rmdir "$CLAUDE_DIRECTORY/commands" 2>/dev/null || true
    rmdir "$CLAUDE_DIRECTORY/skills" 2>/dev/null || true

    echo "uninstalled global Noli Claude Code integration"
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
