#!/bin/sh
# Shared helper for appending Noli's managed guidance without replacing a
# user's existing global agent instructions. Source this file from an
# installer that already uses `set -eu`.

noli_install_managed_guidance() {
    NOLI_GUIDANCE_SOURCE="$1"
    NOLI_GUIDANCE_TARGET="$2"
    NOLI_GUIDANCE_LABEL="$3"
    NOLI_GUIDANCE_START='<!-- noli-global-guidance:start -->'
    NOLI_GUIDANCE_END='<!-- noli-global-guidance:end -->'
    NOLI_GUIDANCE_DIRECTORY=$(dirname "$NOLI_GUIDANCE_TARGET")

    mkdir -p "$NOLI_GUIDANCE_DIRECTORY"
    if [ -L "$NOLI_GUIDANCE_TARGET" ]; then
        echo "refusing to replace symlinked guidance: $NOLI_GUIDANCE_TARGET" >&2
        echo "merge $NOLI_GUIDANCE_SOURCE into its target manually" >&2
        return 1
    fi

    if [ ! -s "$NOLI_GUIDANCE_TARGET" ]; then
        cp "$NOLI_GUIDANCE_SOURCE" "$NOLI_GUIDANCE_TARGET"
        echo "installed $NOLI_GUIDANCE_LABEL at $NOLI_GUIDANCE_TARGET"
        return
    fi

    NOLI_GUIDANCE_START_COUNT=$(grep -cF "$NOLI_GUIDANCE_START" "$NOLI_GUIDANCE_TARGET" || true)
    NOLI_GUIDANCE_END_COUNT=$(grep -cF "$NOLI_GUIDANCE_END" "$NOLI_GUIDANCE_TARGET" || true)
    if [ "$NOLI_GUIDANCE_START_COUNT" -eq 1 ] && [ "$NOLI_GUIDANCE_END_COUNT" -eq 1 ]; then
        echo "$NOLI_GUIDANCE_LABEL already present at $NOLI_GUIDANCE_TARGET"
        return
    fi
    if [ "$NOLI_GUIDANCE_START_COUNT" -ne 0 ] || [ "$NOLI_GUIDANCE_END_COUNT" -ne 0 ]; then
        echo "incomplete or duplicate Noli guidance markers in $NOLI_GUIDANCE_TARGET; repair them before reinstalling" >&2
        return 1
    fi

    NOLI_GUIDANCE_TEMP=$(mktemp "$NOLI_GUIDANCE_DIRECTORY/.noli-guidance.XXXXXX")
    trap 'rm -f "$NOLI_GUIDANCE_TEMP"' EXIT HUP INT TERM
    cp -p "$NOLI_GUIDANCE_TARGET" "$NOLI_GUIDANCE_TEMP"
    printf '\n' >>"$NOLI_GUIDANCE_TEMP"
    sed -n 'p' "$NOLI_GUIDANCE_SOURCE" >>"$NOLI_GUIDANCE_TEMP"
    mv "$NOLI_GUIDANCE_TEMP" "$NOLI_GUIDANCE_TARGET"
    trap - EXIT HUP INT TERM
    echo "added $NOLI_GUIDANCE_LABEL to $NOLI_GUIDANCE_TARGET"
}
