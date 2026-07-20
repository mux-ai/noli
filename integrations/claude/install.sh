#!/bin/sh
# Install Noli's Claude Code integration into one repository or for the
# current user across all repositories. The noli binary must already be on
# PATH (see scripts/install-local.sh).
set -eu

SOURCE="$(cd "$(dirname "$0")" && pwd)"
. "$SOURCE/../shared/install-managed-guidance.sh"

usage() {
    echo "usage: install.sh <target-repository>" >&2
    echo "       install.sh --global" >&2
}

install_skill() {
    CLAUDE_SKILL_DESTINATION="$1/noli-project-knowledge"
    mkdir -p "$CLAUDE_SKILL_DESTINATION"
    cp "$SOURCE/../shared/SKILL.md" "$CLAUDE_SKILL_DESTINATION/SKILL.md"
    cp "$SOURCE/../shared/noli-starter.yaml" "$CLAUDE_SKILL_DESTINATION/noli-starter.yaml"
    cp "$SOURCE/../shared/noli-starter-concepts.yaml" "$CLAUDE_SKILL_DESTINATION/noli-starter-concepts.yaml"
}

install_project() {
    CLAUDE_TARGET="$1"
    if [ ! -d "$CLAUDE_TARGET" ]; then
        echo "target $CLAUDE_TARGET is not a directory" >&2
        exit 1
    fi

    install_skill "$CLAUDE_TARGET/.claude/skills"
    mkdir -p "$CLAUDE_TARGET/.claude/commands"
    cp "$SOURCE/commands/noli-context.md" "$CLAUDE_TARGET/.claude/commands/noli-context.md"

    if [ -f "$CLAUDE_TARGET/CLAUDE.md" ]; then
        echo "note: $CLAUDE_TARGET/CLAUDE.md already exists; merge $SOURCE/CLAUDE.md into it manually" >&2
    else
        cp "$SOURCE/CLAUDE.md" "$CLAUDE_TARGET/CLAUDE.md"
    fi

    echo "installed Noli Claude Code integration into $CLAUDE_TARGET"
}

install_global() {
    : "${HOME:?HOME must be set for --global}"
    CLAUDE_DIRECTORY="${CLAUDE_CONFIG_DIR:-$HOME/.claude}"

    install_skill "$CLAUDE_DIRECTORY/skills"
    mkdir -p "$CLAUDE_DIRECTORY/commands"
    cp "$SOURCE/commands/noli-context.md" "$CLAUDE_DIRECTORY/commands/noli-context.md"
    noli_install_managed_guidance \
        "$SOURCE/../shared/GLOBAL_GUIDANCE.md" \
        "$CLAUDE_DIRECTORY/CLAUDE.md" \
        "global Noli Claude Code guidance"

    echo "installed global Noli Claude Code skill at $CLAUDE_DIRECTORY/skills/noli-project-knowledge"
    echo "installed global /noli-context command at $CLAUDE_DIRECTORY/commands/noli-context.md"
    if ! command -v noli >/dev/null 2>&1; then
        echo "note: noli is not on PATH; run scripts/install-local.sh first" >&2
    fi
}

case "${1:-}" in
--global)
    if [ "$#" -ne 1 ]; then
        usage
        exit 2
    fi
    install_global
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
    if [ "$#" -ne 1 ]; then
        usage
        exit 2
    fi
    install_project "$1"
    ;;
esac
