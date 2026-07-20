#!/bin/sh
# Remove Noli's user-global integrations for Claude Code, Codex, and Pi.
# The CLI and every repository-local knowledge base are deliberately kept.
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

usage() {
    echo "usage: uninstall-global.sh" >&2
}

case "${1:-}" in
"") ;;
-h|--help)
    usage
    exit 0
    ;;
*)
    echo "unknown option: $1" >&2
    usage
    exit 2
    ;;
esac
if [ "$#" -ne 0 ]; then
    usage
    exit 2
fi

sh "$ROOT/integrations/claude/uninstall.sh" --global
sh "$ROOT/integrations/codex/uninstall.sh" --global
sh "$ROOT/integrations/pi/uninstall.sh" --global

echo "uninstalled Noli global integrations for Claude Code, Codex, and Pi"
echo "the noli CLI and repository-local Noli files were preserved"
