#!/bin/sh
# Install the verified Noli Pi extension into a target repository. Pi discovers
# project extensions at .pi/extensions/<name>/index.ts.
set -eu

TARGET="${1:?usage: install.sh <target-repository>}"
SOURCE="$(cd "$(dirname "$0")" && pwd)"
DESTINATION="$TARGET/.pi/extensions/noli"

if [ ! -d "$TARGET" ]; then
    echo "target $TARGET is not a directory" >&2
    exit 1
fi

if [ -d "$TARGET/.pi/extensions/okf" ]; then
    echo "note: remove the deprecated .pi/extensions/okf after reviewing local changes" >&2
fi

mkdir -p "$DESTINATION"
cp "$SOURCE/extension.ts" "$DESTINATION/index.ts"
cp "$SOURCE/runner.ts" "$DESTINATION/runner.ts"

echo "installed Noli Pi extension into $DESTINATION"
echo "Pi requires noli on PATH or an absolute NOLI_BINARY_PATH" >&2
