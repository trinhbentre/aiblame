#!/bin/sh
# Install aiblame from GitHub releases.
#
#   curl -fsSL https://raw.githubusercontent.com/trinhbentre/aiblame/main/install.sh | sh
#
# Environment:
#   AIBLAME_VERSION   release tag (default: latest)
#   AIBLAME_INSTALL   install directory (default: /usr/local/bin if writable, else ~/.local/bin)
set -eu

repo="trinhbentre/aiblame"
version="${AIBLAME_VERSION:-latest}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux|darwin) ;;
  mingw*|msys*|cygwin*) os=windows ;;
  *) echo "aiblame: unsupported OS: $os" >&2; exit 1 ;;
esac
arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) echo "aiblame: unsupported architecture: $arch" >&2; exit 1 ;;
esac

if [ "$version" = "latest" ]; then
  base="https://github.com/$repo/releases/latest/download"
else
  base="https://github.com/$repo/releases/download/$version"
fi

tmp=$(mktemp -d 2>/dev/null || mktemp -d -t aiblame)
cleanup() { rm -rf "$tmp"; }
trap cleanup EXIT

fetch() {
  if command -v curl >/dev/null 2>&1; then curl -fsSL "$1" -o "$2"
  elif command -v wget >/dev/null 2>&1; then wget -q "$1" -O "$2"
  else echo "aiblame: need curl or wget" >&2; exit 1; fi
}

if [ "$os" = windows ]; then
  asset="aiblame_${os}_${arch}.zip"
else
  asset="aiblame_${os}_${arch}.tar.gz"
fi
echo "downloading $base/$asset"
fetch "$base/$asset" "$tmp/$asset"
fetch "$base/SHA256SUMS" "$tmp/SHA256SUMS"

cd "$tmp"
if command -v sha256sum >/dev/null 2>&1; then
  grep " $asset\$" SHA256SUMS | sha256sum -c - >/dev/null
elif command -v shasum >/dev/null 2>&1; then
  grep " $asset\$" SHA256SUMS | shasum -a 256 -c - >/dev/null
elif command -v openssl >/dev/null 2>&1; then
  want=$(grep " $asset\$" SHA256SUMS | cut -d' ' -f1)
  got=$(openssl dgst -sha256 "$asset" | sed 's/.*= //')
  [ "$want" = "$got" ] || { echo "aiblame: checksum mismatch for $asset" >&2; exit 1; }
else
  echo "aiblame: no sha256sum, shasum or openssl found; refusing to install an unverified binary" >&2
  echo "        set AIBLAME_SKIP_VERIFY=1 to override" >&2
  [ -n "${AIBLAME_SKIP_VERIFY:-}" ] || exit 1
fi

if [ "$os" = windows ]; then
  unzip -oq "$asset"
  bin="aiblame_${os}_${arch}.exe"
  target_name="aiblame.exe"
else
  tar -xzf "$asset"
  bin="aiblame_${os}_${arch}"
  target_name="aiblame"
fi
chmod +x "$bin"

dest="${AIBLAME_INSTALL:-}"
if [ -z "$dest" ]; then
  if [ -w /usr/local/bin ]; then dest=/usr/local/bin; else dest="$HOME/.local/bin"; fi
fi
mkdir -p "$dest"
mv "$bin" "$dest/$target_name"
echo "installed $dest/$target_name"
"$dest/$target_name" version || true
case ":$PATH:" in
  *":$dest:"*) ;;
  *) echo "note: add $dest to your PATH" ;;
esac
