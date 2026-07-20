#!/bin/sh
# Shared helpers for removing only Noli-managed global agent files. Source
# this file from an uninstaller that already uses `set -eu`.

NOLI_UNINSTALL_GUIDANCE_START='<!-- noli-global-guidance:start -->'
NOLI_UNINSTALL_GUIDANCE_END='<!-- noli-global-guidance:end -->'

noli_guidance_has_managed_block() {
    NOLI_UNINSTALL_CHECK_TARGET="$1"
    [ -f "$NOLI_UNINSTALL_CHECK_TARGET" ] || return 1
    [ ! -L "$NOLI_UNINSTALL_CHECK_TARGET" ] || return 1
    [ "$(grep -cxF "$NOLI_UNINSTALL_GUIDANCE_START" "$NOLI_UNINSTALL_CHECK_TARGET" || true)" -eq 1 ] &&
        [ "$(grep -cxF "$NOLI_UNINSTALL_GUIDANCE_END" "$NOLI_UNINSTALL_CHECK_TARGET" || true)" -eq 1 ]
}

noli_codex_integration_present() {
    NOLI_UNINSTALL_CODEX_DIRECTORY="$1"
    noli_guidance_has_managed_block "$NOLI_UNINSTALL_CODEX_DIRECTORY/AGENTS.override.md" ||
        noli_guidance_has_managed_block "$NOLI_UNINSTALL_CODEX_DIRECTORY/AGENTS.md"
}

noli_pi_integration_present() {
    NOLI_UNINSTALL_PI_DIRECTORY="$1"
    [ -d "$NOLI_UNINSTALL_PI_DIRECTORY/extensions/noli" ] ||
        noli_guidance_has_managed_block "$NOLI_UNINSTALL_PI_DIRECTORY/AGENTS.md"
}

noli_remove_managed_guidance() {
    NOLI_UNINSTALL_GUIDANCE_TARGET="$1"
    NOLI_UNINSTALL_GUIDANCE_SOURCE="$2"
    NOLI_UNINSTALL_GUIDANCE_LABEL="$3"

    if [ ! -e "$NOLI_UNINSTALL_GUIDANCE_TARGET" ] && [ ! -L "$NOLI_UNINSTALL_GUIDANCE_TARGET" ]; then
        echo "$NOLI_UNINSTALL_GUIDANCE_LABEL is not installed at $NOLI_UNINSTALL_GUIDANCE_TARGET"
        return
    fi
    if [ -L "$NOLI_UNINSTALL_GUIDANCE_TARGET" ]; then
        echo "refusing to edit symlinked guidance: $NOLI_UNINSTALL_GUIDANCE_TARGET" >&2
        return 1
    fi
    if [ ! -f "$NOLI_UNINSTALL_GUIDANCE_TARGET" ]; then
        echo "preserved non-file guidance path: $NOLI_UNINSTALL_GUIDANCE_TARGET" >&2
        return
    fi

    NOLI_UNINSTALL_START_COUNT=$(grep -cxF "$NOLI_UNINSTALL_GUIDANCE_START" "$NOLI_UNINSTALL_GUIDANCE_TARGET" || true)
    NOLI_UNINSTALL_END_COUNT=$(grep -cxF "$NOLI_UNINSTALL_GUIDANCE_END" "$NOLI_UNINSTALL_GUIDANCE_TARGET" || true)
    if [ "$NOLI_UNINSTALL_START_COUNT" -eq 0 ] && [ "$NOLI_UNINSTALL_END_COUNT" -eq 0 ]; then
        echo "$NOLI_UNINSTALL_GUIDANCE_LABEL is not present in $NOLI_UNINSTALL_GUIDANCE_TARGET"
        return
    fi
    if [ "$NOLI_UNINSTALL_START_COUNT" -ne 1 ] || [ "$NOLI_UNINSTALL_END_COUNT" -ne 1 ]; then
        echo "incomplete or duplicate Noli guidance markers in $NOLI_UNINSTALL_GUIDANCE_TARGET; preserved file" >&2
        return 1
    fi

    NOLI_UNINSTALL_START_LINE=$(grep -nxF "$NOLI_UNINSTALL_GUIDANCE_START" "$NOLI_UNINSTALL_GUIDANCE_TARGET" | cut -d: -f1)
    NOLI_UNINSTALL_END_LINE=$(grep -nxF "$NOLI_UNINSTALL_GUIDANCE_END" "$NOLI_UNINSTALL_GUIDANCE_TARGET" | cut -d: -f1)
    if [ "$NOLI_UNINSTALL_START_LINE" -ge "$NOLI_UNINSTALL_END_LINE" ]; then
        echo "misordered Noli guidance markers in $NOLI_UNINSTALL_GUIDANCE_TARGET; preserved file" >&2
        return 1
    fi

    if cmp -s "$NOLI_UNINSTALL_GUIDANCE_SOURCE" "$NOLI_UNINSTALL_GUIDANCE_TARGET"; then
        rm "$NOLI_UNINSTALL_GUIDANCE_TARGET"
        echo "removed $NOLI_UNINSTALL_GUIDANCE_LABEL file at $NOLI_UNINSTALL_GUIDANCE_TARGET"
        return
    fi

    NOLI_UNINSTALL_GUIDANCE_DIRECTORY=$(dirname "$NOLI_UNINSTALL_GUIDANCE_TARGET")
    NOLI_UNINSTALL_GUIDANCE_TEMP=$(mktemp "$NOLI_UNINSTALL_GUIDANCE_DIRECTORY/.noli-uninstall.XXXXXX")
    trap 'rm -f "$NOLI_UNINSTALL_GUIDANCE_TEMP"' EXIT HUP INT TERM
    cp -p "$NOLI_UNINSTALL_GUIDANCE_TARGET" "$NOLI_UNINSTALL_GUIDANCE_TEMP"
    awk -v start="$NOLI_UNINSTALL_GUIDANCE_START" -v end="$NOLI_UNINSTALL_GUIDANCE_END" '
        $0 == start {
            if (have_pending && pending != "") print pending
            have_pending = 0
            skipping = 1
            next
        }
        skipping {
            if ($0 == end) skipping = 0
            next
        }
        {
            if (have_pending) print pending
            pending = $0
            have_pending = 1
        }
        END {
            if (have_pending) print pending
        }
    ' "$NOLI_UNINSTALL_GUIDANCE_TARGET" >"$NOLI_UNINSTALL_GUIDANCE_TEMP"
    mv "$NOLI_UNINSTALL_GUIDANCE_TEMP" "$NOLI_UNINSTALL_GUIDANCE_TARGET"
    trap - EXIT HUP INT TERM
    echo "removed $NOLI_UNINSTALL_GUIDANCE_LABEL block from $NOLI_UNINSTALL_GUIDANCE_TARGET"
}

noli_remove_managed_file() {
    NOLI_UNINSTALL_FILE_TARGET="$1"
    NOLI_UNINSTALL_FILE_SOURCE="$2"
    NOLI_UNINSTALL_FILE_LABEL="$3"

    if [ ! -e "$NOLI_UNINSTALL_FILE_TARGET" ] && [ ! -L "$NOLI_UNINSTALL_FILE_TARGET" ]; then
        return
    fi
    if [ -L "$NOLI_UNINSTALL_FILE_TARGET" ] || [ ! -f "$NOLI_UNINSTALL_FILE_TARGET" ]; then
        echo "preserved unexpected $NOLI_UNINSTALL_FILE_LABEL at $NOLI_UNINSTALL_FILE_TARGET" >&2
        return
    fi
    if ! cmp -s "$NOLI_UNINSTALL_FILE_SOURCE" "$NOLI_UNINSTALL_FILE_TARGET"; then
        echo "preserved modified $NOLI_UNINSTALL_FILE_LABEL at $NOLI_UNINSTALL_FILE_TARGET" >&2
        return
    fi
    rm "$NOLI_UNINSTALL_FILE_TARGET"
    echo "removed $NOLI_UNINSTALL_FILE_LABEL at $NOLI_UNINSTALL_FILE_TARGET"
}

noli_remove_managed_skill() {
    NOLI_UNINSTALL_SKILL_ROOT="$1"
    NOLI_UNINSTALL_SKILL_SOURCE="$2"
    NOLI_UNINSTALL_SKILL_DIRECTORY="$NOLI_UNINSTALL_SKILL_ROOT/noli-project-knowledge"

    noli_remove_managed_file "$NOLI_UNINSTALL_SKILL_DIRECTORY/SKILL.md" "$NOLI_UNINSTALL_SKILL_SOURCE/SKILL.md" "Noli skill"
    noli_remove_managed_file "$NOLI_UNINSTALL_SKILL_DIRECTORY/noli-starter.yaml" "$NOLI_UNINSTALL_SKILL_SOURCE/noli-starter.yaml" "Noli starter config"
    noli_remove_managed_file "$NOLI_UNINSTALL_SKILL_DIRECTORY/noli-starter-concepts.yaml" "$NOLI_UNINSTALL_SKILL_SOURCE/noli-starter-concepts.yaml" "Noli starter concepts"
    rmdir "$NOLI_UNINSTALL_SKILL_DIRECTORY" 2>/dev/null || true
}
