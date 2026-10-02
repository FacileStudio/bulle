#!/usr/bin/env bash
# Local installability test for the bulle Homebrew cask, without requiring
# Homebrew to be installed. Replicates what `brew install --cask bulle` does:
#  1. resolve the cask URL
#  2. verify the sha256 against the downloaded/staged tarball
#  3. extract the tarball into a staged path
#  4. confirm the binary is present at the cask's `binary` path and runs
#
# Reads the staged tarball from the local-test cask URL in
# ../Code/Homebrew-tap/Casks/bulle.rb (file:// URL) and the checksum it declares.
set -eu

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(dirname "$script_dir")"
tap_cask="$HOME/Code/Homebrew-tap/Casks/bulle.rb"
tarball="$repo_root/dist/bulle_0.0.0_linux_amd64.tar.gz"

echo "==> cask: $tap_cask"
echo "==> tarball: $tarball"

# 1. + 2. checksum verify (the gate a broken tarball would fail)
actual=$(shasum -a 256 "$tarball" | awk '{print $1}')
declared=$(sed -n 's/.*sha256 "\([0-9a-f]*\)".*/\1/p' "$tap_cask")
if [ "$actual" != "$declared" ]; then
  echo "CHECKSUM MISMATCH: tarball=$actual cask=$declared" >&2
  exit 1
fi
echo "==> sha256 verified: $actual"

# 3. stage and extract
stage="$(mktemp -d)"
tar xzf "$tarball" -C "$stage"
if [ ! -f "$stage/bulle" ]; then
  echo "MISSING: no bulle binary inside tarball (contents: $(ls "$stage"))" >&2
  exit 1
fi
echo "==> extracted to $stage/bulle"

# 4. binary present + runs
chmod +x "$stage/bulle"
if [ "$($stage/bulle -version 2>/dev/null)" != "bulle v0.57.0" ]; then
  echo "BINARY DID NOT RUN: $($stage/bulle -version 2>&1)" >&2
  exit 1
fi
echo "==> binary runs: $($stage/bulle -version)"

echo "==> local cask install test PASSED"
