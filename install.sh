#!/bin/bash

set -e

REPO="filipexyz/notif"
INSTALL_DIR="$HOME/.notif/bin"
BINARY_NAME="notif"

# Check for required dependencies
DOWNLOADER=""
if command -v curl >/dev/null 2>&1; then
    DOWNLOADER="curl"
elif command -v wget >/dev/null 2>&1; then
    DOWNLOADER="wget"
else
    echo "Either curl or wget is required but neither is installed" >&2
    exit 1
fi

# Download function that works with both curl and wget
download_file() {
    local url="$1"
    local output="$2"

    if [ "$DOWNLOADER" = "curl" ]; then
        if [ -n "$output" ]; then
            curl -fsSL -o "$output" "$url"
        else
            curl -fsSL "$url"
        fi
    elif [ "$DOWNLOADER" = "wget" ]; then
        if [ -n "$output" ]; then
            wget -q -O "$output" "$url"
        else
            wget -q -O - "$url"
        fi
    else
        return 1
    fi
}

# Detect platform
case "$(uname -s)" in
    Darwin) OS="darwin" ;;
    Linux) OS="linux" ;;
    MINGW*|MSYS*|CYGWIN*) OS="windows" ;;
    *) echo "Unsupported OS: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
    x86_64|amd64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

# Get latest version from GitHub
get_latest_version() {
    download_file "https://api.github.com/repos/${REPO}/releases/latest" "" |
        grep '"tag_name":' |
        sed -E 's/.*"v([^"]+)".*/\1/'
}

VERSION="${VERSION:-$(get_latest_version)}"

if [ -z "$VERSION" ]; then
    echo "Error: Could not determine latest version" >&2
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

# Create install directory
mkdir -p "$INSTALL_DIR"

# Download binary
BINARY_PATH="$INSTALL_DIR/${BINARY_NAME}${EXT}"
if ! download_file "$URL" "$BINARY_PATH"; then
    echo "Download failed" >&2
    rm -f "$BINARY_PATH"
    exit 1
fi

chmod +x "$BINARY_PATH"

# Clear macOS quarantine attributes to prevent Gatekeeper blocking
if [ "$OS" = "darwin" ]; then
    xattr -cr "$HOME/.notif" 2>/dev/null || true
fi

# Setup shell integration
setup_path() {
    local shell_config=""
    local path_line="export PATH=\"\$HOME/.notif/bin:\$PATH\""

    # Detect shell and config file
    case "$SHELL" in
        */zsh) shell_config="$HOME/.zshrc" ;;
        */bash)
            if [ -f "$HOME/.bashrc" ]; then
                shell_config="$HOME/.bashrc"
            elif [ -f "$HOME/.bash_profile" ]; then
                shell_config="$HOME/.bash_profile"
            fi
            ;;
        */fish)
            shell_config="$HOME/.config/fish/config.fish"
            path_line="set -gx PATH \$HOME/.notif/bin \$PATH"
            ;;
    esac

    if [ -n "$shell_config" ]; then
        # Check if already in config
        if ! grep -q ".notif/bin" "$shell_config" 2>/dev/null; then
            echo "" >> "$shell_config"
            echo "# notif.sh CLI" >> "$shell_config"
            echo "$path_line" >> "$shell_config"
            echo "Added $INSTALL_DIR to PATH in $shell_config"
        fi
    fi
}

setup_path

echo ""
echo "Installation complete!"
echo ""
echo "Restart your shell or run:"
echo "  export PATH=\"\$HOME/.notif/bin:\$PATH\""
echo ""
echo "Get started:"
echo "  notif --help"
