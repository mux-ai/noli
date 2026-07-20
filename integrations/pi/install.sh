#!/bin/sh
# Install Noli's verified Pi extension and shared skill into one repository or
# for the current user across all repositories. The noli binary must already
# be on PATH (see scripts/install-local.sh).
set -eu

SOURCE="$(cd "$(dirname "$0")" && pwd)"
. "$SOURCE/../shared/install-managed-guidance.sh"

usage() {
    echo "usage: install.sh <target-repository>" >&2
    echo "       install.sh --global" >&2
}

install_extension() {
    PI_EXTENSION_DESTINATION="$1/noli"
    mkdir -p "$PI_EXTENSION_DESTINATION"
    cp "$SOURCE/extension.ts" "$PI_EXTENSION_DESTINATION/index.ts"
    cp "$SOURCE/runner.ts" "$PI_EXTENSION_DESTINATION/runner.ts"
}

install_skill() {
    PI_SKILL_DESTINATION="$1/noli-project-knowledge"
    mkdir -p "$PI_SKILL_DESTINATION"
    cp "$SOURCE/../shared/SKILL.md" "$PI_SKILL_DESTINATION/SKILL.md"
    cp "$SOURCE/../shared/noli-starter.yaml" "$PI_SKILL_DESTINATION/noli-starter.yaml"
    cp "$SOURCE/../shared/noli-starter-concepts.yaml" "$PI_SKILL_DESTINATION/noli-starter-concepts.yaml"
}

install_project() {
    PI_TARGET="$1"
    if [ ! -d "$PI_TARGET" ]; then
        echo "target $PI_TARGET is not a directory" >&2
        exit 1
    fi

    if [ -d "$PI_TARGET/.pi/extensions/okf" ]; then
        echo "note: remove the deprecated .pi/extensions/okf after reviewing local changes" >&2
    fi

    install_extension "$PI_TARGET/.pi/extensions"
    install_skill "$PI_TARGET/.agents/skills"

    echo "installed Noli Pi extension into $PI_TARGET/.pi/extensions/noli"
    echo "installed shared Noli skill into $PI_TARGET/.agents/skills/noli-project-knowledge"
    echo "Pi requires noli on PATH or an absolute NOLI_BINARY_PATH" >&2
}

install_global() {
    : "${HOME:?HOME must be set for --global}"
    PI_AGENT_DIRECTORY="${PI_CODING_AGENT_DIR:-$HOME/.pi/agent}"
    PI_SHARED_SKILLS="${NOLI_PI_SKILLS_DIR:-${NOLI_AGENT_SKILLS_DIR:-$HOME/.agents/skills}}"

    install_extension "$PI_AGENT_DIRECTORY/extensions"
    install_skill "$PI_SHARED_SKILLS"
    noli_install_managed_guidance \
        "$SOURCE/../shared/GLOBAL_GUIDANCE.md" \
        "$PI_AGENT_DIRECTORY/AGENTS.md" \
        "global Noli Pi guidance"

    echo "installed global Noli Pi extension at $PI_AGENT_DIRECTORY/extensions/noli"
    echo "installed shared Noli skill at $PI_SHARED_SKILLS/noli-project-knowledge"
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
