#!/bin/sh
# Build and install Noli's primary binary plus the deprecated okf alias.
# Override the destination with NOLI_INSTALL_DIR. OKF_INSTALL_DIR remains a
# deprecated fallback for existing automation.
set -eu

cd "$(dirname "$0")/.."
INSTALL_DIR="${NOLI_INSTALL_DIR:-${OKF_INSTALL_DIR:-$HOME/.local/bin}}"
mkdir -p "$INSTALL_DIR"

GOCACHE="${GOCACHE:-${TMPDIR:-/tmp}/noli-gocache}" \
    go build -buildvcs=false -o "$INSTALL_DIR/noli" ./cmd/noli
GOCACHE="${GOCACHE:-${TMPDIR:-/tmp}/noli-gocache}" \
    go build -buildvcs=false -o "$INSTALL_DIR/okf" ./cmd/okf

echo "installed Noli CLI at $INSTALL_DIR/noli"
echo "installed deprecated compatibility alias at $INSTALL_DIR/okf"
case ":$PATH:" in
*":$INSTALL_DIR:"*) ;;
*) echo "note: $INSTALL_DIR is not on PATH" >&2 ;;
esac
