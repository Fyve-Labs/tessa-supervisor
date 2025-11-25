#!/usr/bin/env sh
set -eu

# install.sh — Install tessad binary (Linux arm64)
#
# This script downloads the linux/arm64 release artifact from GitHub and
# installs it into a directory on your PATH (default: /usr/local/bin).
#
# Usage (quick):
#   curl -fsSL https://raw.githubusercontent.com/Fyve-Labs/tessa-daemon/main/install.sh | sudo sh
#
# Specify version:
#   curl -fsSL https://raw.githubusercontent.com/Fyve-Labs/tessa-daemon/main/install.sh \
#     | sudo VERSION=v1.2.3 sh
#
# Environment variables:
#   VERSION   - Tag to install (e.g., v1.2.3). If unset, installs the latest release.
#   BIN_DIR   - Install destination (default: /usr/local/bin)
#   REPO      - GitHub repo (owner/name). Default: Fyve-Labs/tessa-daemon
#   NO_SUDO   - If set (non-empty), do not attempt to use sudo.
#   VERIFY    - If set to "0", skip checksum verification.
#

REPO="${REPO:-Fyve-Labs/tessa-daemon}"
BIN_DIR="${BIN_DIR:-/usr/local/bin}"
VERIFY="${VERIFY:-1}"

# Detect OS/ARCH and ensure supported target
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "This installer currently supports only Linux arm64 (aarch64). Detected ARCH=$ARCH" >&2; exit 1 ;;
 esac

if [ "$OS" != "linux" ]; then
  echo "This installer currently supports only Linux. Detected OS=$OS" >&2
  exit 1
fi

# Determine version/tag
TAG="${VERSION:-}"
if [ -z "$TAG" ]; then
  # Query GitHub API for the latest release tag
  API_URL="https://api.github.com/repos/${REPO}/releases/latest"
  echo "Fetching latest release tag from ${API_URL} ..." >&2
  TAG=$(curl -fsSL "$API_URL" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name"\s*:\s*"([^"]+)".*/\1/') || true
  if [ -z "$TAG" ]; then
    echo "Failed to determine latest release tag from GitHub API." >&2
    exit 1
  fi
fi
# normalize: ensure leading v
case "$TAG" in
  v*) : ;;
  *) TAG="v${TAG}" ;;
esac

ASSET="wifi-provisioner_${TAG#v}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"
URL_TGZ="${BASE_URL}/${ASSET}"
URL_SUM="${BASE_URL}/checksums.txt"

TMP_DIR=$(mktemp -d)
cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

cd "$TMP_DIR"

echo "Downloading ${URL_TGZ} ..." >&2
curl -fL --retry 3 --retry-delay 2 -o "$ASSET" "$URL_TGZ"

if [ "$VERIFY" != "0" ]; then
  echo "Downloading checksums and verifying artifact ..." >&2
  curl -fL --retry 3 --retry-delay 2 -o checksums.txt "$URL_SUM"
  if command -v sha256sum >/dev/null 2>&1; then
    grep "  ${ASSET}$" checksums.txt | sha256sum -c -
  elif command -v shasum >/dev/null 2>&1; then
    SUM=$(grep "  ${ASSET}$" checksums.txt | awk '{print $1}')
    echo "${SUM}  ${ASSET}" | shasum -a 256 -c -
  else
    echo "No sha256 tool found. Skipping checksum verification." >&2
  fi
else
  echo "Checksum verification disabled (VERIFY=0)." >&2
fi

# Extract and install
mkdir -p extracted

tar -xzf "$ASSET" -C extracted

# The archive should contain the binary named 'tessad'
BIN_PATH=$(find extracted -type f -name 'tessad' | head -n1 || true)
if [ -z "$BIN_PATH" ]; then
  echo "Binary 'tessad' not found in archive." >&2
  exit 1
fi
chmod +x "$BIN_PATH"

install_file() {
  mkdir -p "$BIN_DIR"
  cp "$BIN_PATH" "$BIN_DIR/tessad"
}

NEED_SUDO=0
if [ ! -w "$BIN_DIR" ]; then
  NEED_SUDO=1
fi

if [ "$NEED_SUDO" -eq 1 ] && [ -z "${NO_SUDO:-}" ]; then
  if command -v sudo >/dev/null 2>&1; then
    echo "Installing to $BIN_DIR with sudo ..." >&2
    sudo sh -c "mkdir -p '$BIN_DIR' && cp '$BIN_PATH' '$BIN_DIR/tessad' && chmod 755 '$BIN_DIR/tessad'"
  else
    echo "No write permission to $BIN_DIR and 'sudo' not found. Set BIN_DIR to a writable path or run as root." >&2
    exit 1
  fi
else
  echo "Installing to $BIN_DIR ..." >&2
  install_file
  chmod 755 "$BIN_DIR/tessad"
fi

# Show version if the binary supports it (no flags implemented yet, so just confirm presence)
if command -v "$BIN_DIR/tessad" >/dev/null 2>&1; then
  echo "Installed tessad to $BIN_DIR/tessad (tag $TAG)."
else
  echo "Install finished, but $BIN_DIR may not be on your PATH." >&2
fi
