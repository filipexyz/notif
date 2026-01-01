#!/bin/sh
set -e

# notif.sh CLI installer
# Usage: curl -fsSL https://raw.githubusercontent.com/filipexyz/notif/main/install.sh | sh

REPO="filipexyz/notif"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="notif"

# Detect OS
OS="$(uname -s)"
case "$OS" in
  Linux*)  OS=linux ;;
  Darwin*) OS=darwin ;;
  MINGW*|MSYS*|CYGWIN*) OS=windows ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Get latest version from GitHub
get_latest_version() {
  curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
    grep '"tag_name":' |
    sed -E 's/.*"v([^"]+)".*/\1/'
}

VERSION="${VERSION:-$(get_latest_version)}"

if [ -z "$VERSION" ]; then
  echo "Error: Could not determine latest version"
  exit 1
fi

# Build download URL
EXT=""
if [ "$OS" = "windows" ]; then
  EXT=".exe"
fi

FILENAME="${BINARY_NAME}-${OS}-${ARCH}${EXT}"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${FILENAME}"

echo "Installing notif v${VERSION} for ${OS}/${ARCH}..."

# Create temp directory
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

# Download binary
curl -fsSL "$URL" -o "${TMP_DIR}/${BINARY_NAME}${EXT}"

# Install binary
if [ -w "$INSTALL_DIR" ]; then
  mv "${TMP_DIR}/${BINARY_NAME}${EXT}" "${INSTALL_DIR}/${BINARY_NAME}${EXT}"
  chmod +x "${INSTALL_DIR}/${BINARY_NAME}${EXT}"
else
  echo "Need sudo to install to ${INSTALL_DIR}"
  sudo mv "${TMP_DIR}/${BINARY_NAME}${EXT}" "${INSTALL_DIR}/${BINARY_NAME}${EXT}"
  sudo chmod +x "${INSTALL_DIR}/${BINARY_NAME}${EXT}"
fi

echo "Successfully installed notif to ${INSTALL_DIR}/${BINARY_NAME}"
echo ""
echo "Get started:"
echo "  notif --help"
