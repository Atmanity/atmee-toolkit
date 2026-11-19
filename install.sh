#!/bin/sh
# Installs the atmee CLI from the latest Atmanity/atmee-toolkit release: a
# prebuilt binary, checked against the release's checksums. No Go needed.
#
#   curl -fsSL https://raw.githubusercontent.com/Atmanity/atmee-toolkit/main/install.sh | sh
#
# ATMEE_INSTALL_DIR  where the binary goes (default: next to the atmee already
#                    on PATH when that folder is writable, else ~/.local/bin)
# ATMEE_VERSION      a release to install instead of the latest, such as 0.10.18
#
# Run it again to upgrade. On Windows, download atmee_windows_amd64.zip from
# the release page instead.
set -eu

repo="Atmanity/atmee-toolkit"
# Upgrade the copy that runs: an older one first on PATH (a go install in
# ~/go/bin, say) would otherwise keep answering.
existing="$(command -v atmee 2> /dev/null || true)"
if [ -n "${ATMEE_INSTALL_DIR:-}" ]; then
  dir="$ATMEE_INSTALL_DIR"
elif [ -n "$existing" ] && [ -w "$(dirname "$existing")" ]; then
  dir="$(dirname "$existing")"
else
  dir="$HOME/.local/bin"
fi
if [ -n "${ATMEE_DOWNLOAD_BASE:-}" ]; then
  base="$ATMEE_DOWNLOAD_BASE"
elif [ -n "${ATMEE_VERSION:-}" ]; then
  base="https://github.com/$repo/releases/download/atmee--v${ATMEE_VERSION#v}"
else
  base="https://github.com/$repo/releases/latest/download"
fi

fail() { echo "atmee install: $*" >&2; exit 1; }

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) fail "no prebuilt atmee for $(uname -s); download it from https://github.com/$repo/releases/latest" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) fail "no prebuilt atmee for $(uname -m)" ;;
esac
file="atmee_${os}_${arch}.tar.gz"

if command -v curl > /dev/null 2>&1; then
  get() { curl -fsSL "$1" -o "$2"; }
elif command -v wget > /dev/null 2>&1; then
  get() { wget -q "$1" -O "$2"; }
else
  fail "needs curl or wget"
fi
if command -v sha256sum > /dev/null 2>&1; then
  sum() { sha256sum "$1" | awk '{ print $1 }'; }
elif command -v shasum > /dev/null 2>&1; then
  sum() { shasum -a 256 "$1" | awk '{ print $1 }'; }
else
  fail "needs sha256sum or shasum to check the download"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
get "$base/$file" "$tmp/$file" || fail "could not download $base/$file"
get "$base/checksums.txt" "$tmp/checksums.txt" || fail "could not download $base/checksums.txt"
want="$(awk -v f="$file" '$2 == f || $2 == "*" f { print $1 }' "$tmp/checksums.txt")"
[ -n "$want" ] || fail "checksums.txt has no line for $file"
[ "$(sum "$tmp/$file")" = "$want" ] || fail "$file does not match its checksum; nothing installed"

tar -xzf "$tmp/$file" -C "$tmp" atmee
mkdir -p "$dir"
# Replace by rename, so a running atmee is never overwritten in place.
mv "$tmp/atmee" "$dir/.atmee.new"
chmod 755 "$dir/.atmee.new"
mv "$dir/.atmee.new" "$dir/atmee"

echo "Installed $("$dir/atmee" --version 2>/dev/null || echo atmee) to $dir/atmee"
case ":$PATH:" in
  *":$dir:"*)
    now="$(command -v atmee 2> /dev/null || true)"
    if [ -n "$now" ] && [ "$now" != "$dir/atmee" ]; then
      echo "Another atmee comes first on your PATH: $now. Remove it, or put $dir before it."
    fi ;;
  *) echo "$dir is not on your PATH. Add it, for example: export PATH=\"$dir:\$PATH\"" ;;
esac
