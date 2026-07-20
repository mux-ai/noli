#!/bin/sh
# Install Noli's Codex integration either into one repository or for the
# current user across repositories. The noli binary must already be on PATH
# (see scripts/install-local.sh).
set -eu

SOURCE="$(cd "$(dirname "$0")" && pwd)"
GLOBAL_START='<!-- noli-global-guidance:start -->'
GLOBAL_END='<!-- noli-global-guidance:end -->'

usage() {
    echo "usage: install.sh <target-repository>" >&2
    echo "       install.sh --global" >&2
}

install_skill() {
    SKILL_DESTINATION="$1/noli-project-knowledge"
    mkdir -p "$SKILL_DESTINATION"
    cp "$SOURCE/../shared/SKILL.md" "$SKILL_DESTINATION/SKILL.md"
    cp "$SOURCE/../shared/noli-starter.yaml" "$SKILL_DESTINATION/noli-starter.yaml"
    cp "$SOURCE/../shared/noli-starter-concepts.yaml" "$SKILL_DESTINATION/noli-starter-concepts.yaml"
}

install_project() {
    TARGET="$1"
    if [ ! -d "$TARGET" ]; then
        echo "target $TARGET is not a directory" >&2
        exit 1
    fi

    if [ -d "$TARGET/.agents/skills/okf-project-knowledge" ]; then
        echo "note: remove the deprecated .agents/skills/okf-project-knowledge after reviewing local changes" >&2
    fi

    install_skill "$TARGET/.agents/skills"

    if [ -f "$TARGET/AGENTS.md" ]; then
        echo "note: $TARGET/AGENTS.md already exists; merge $SOURCE/AGENTS.md into it manually" >&2
    else
        cp "$SOURCE/AGENTS.md" "$TARGET/AGENTS.md"
    fi

    echo "installed Noli Codex integration into $TARGET"
}

install_global_guidance() {
    CODEX_DIRECTORY="$1"
    mkdir -p "$CODEX_DIRECTORY"

    if [ -s "$CODEX_DIRECTORY/AGENTS.override.md" ]; then
        GLOBAL_AGENTS="$CODEX_DIRECTORY/AGENTS.override.md"
    else
        GLOBAL_AGENTS="$CODEX_DIRECTORY/AGENTS.md"
    fi

    if [ -L "$GLOBAL_AGENTS" ]; then
        echo "refusing to replace symlinked global guidance: $GLOBAL_AGENTS" >&2
        echo "merge $SOURCE/GLOBAL_AGENTS.md into its target manually" >&2
        exit 1
    fi

    if [ ! -s "$GLOBAL_AGENTS" ]; then
        cp "$SOURCE/GLOBAL_AGENTS.md" "$GLOBAL_AGENTS"
        echo "installed global Noli guidance at $GLOBAL_AGENTS"
        return
    fi

    START_COUNT=$(grep -cF "$GLOBAL_START" "$GLOBAL_AGENTS" || true)
    END_COUNT=$(grep -cF "$GLOBAL_END" "$GLOBAL_AGENTS" || true)
    if [ "$START_COUNT" -eq 1 ] && [ "$END_COUNT" -eq 1 ]; then
        echo "global Noli guidance already present at $GLOBAL_AGENTS"
        return
    fi
    if [ "$START_COUNT" -ne 0 ] || [ "$END_COUNT" -ne 0 ]; then
        echo "incomplete or duplicate Noli guidance markers in $GLOBAL_AGENTS; repair them before reinstalling" >&2
        exit 1
    fi

    GLOBAL_TEMP=$(mktemp "$CODEX_DIRECTORY/.noli-agents.XXXXXX")
    trap 'rm -f "$GLOBAL_TEMP"' EXIT HUP INT TERM
    cp "$GLOBAL_AGENTS" "$GLOBAL_TEMP"
    printf '\n' >>"$GLOBAL_TEMP"
    sed -n 'p' "$SOURCE/GLOBAL_AGENTS.md" >>"$GLOBAL_TEMP"
    mv "$GLOBAL_TEMP" "$GLOBAL_AGENTS"
    trap - EXIT HUP INT TERM
    echo "added global Noli guidance to $GLOBAL_AGENTS"
}

install_global() {
    : "${HOME:?HOME must be set for --global}"
    USER_SKILLS="${NOLI_CODEX_SKILLS_DIR:-$HOME/.agents/skills}"
    CODEX_DIRECTORY="${CODEX_HOME:-$HOME/.codex}"

    install_skill "$USER_SKILLS"
    install_global_guidance "$CODEX_DIRECTORY"

    echo "installed global Noli Codex skill at $USER_SKILLS/noli-project-knowledge"
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
