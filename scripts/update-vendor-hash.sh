#!/usr/bin/env bash
# Updates the vendorHash in flake.nix by attempting a build with a fake hash
# and extracting the real hash from the error message.
set -euo pipefail

FLAKE="$(git rev-parse --show-toplevel)/flake.nix"

# Temporarily set fakeHash
sed -i 's|vendorHash = ".*"|vendorHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="|' "$FLAKE"

# Build and capture the real hash from the error
REAL_HASH=$(nix build .#default 2>&1 | grep "got:" | awk '{print $2}') || true

if [ -z "$REAL_HASH" ]; then
  echo "Error: could not determine vendor hash. Is the build failing for another reason?" >&2
  git checkout "$FLAKE"
  exit 1
fi

# Replace with real hash
sed -i "s|vendorHash = \".*\"|vendorHash = \"$REAL_HASH\"|" "$FLAKE"

echo "Updated vendorHash to $REAL_HASH"
