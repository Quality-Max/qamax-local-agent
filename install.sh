#!/bin/bash
# QualityMax Local Agent Installer for macOS/Linux
# Downloads the latest release from GitHub

set -e

REPO="Quality-Max/qamax-local-agent"

echo "QualityMax Local Agent Installer"
echo "================================"
echo ""

# Detect OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    arm64)   ARCH="arm64" ;;
    *)
        echo "Error: Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

case "$OS" in
    darwin|linux) ;;
    *)
        echo "Error: Unsupported OS: $OS"
        echo "For Windows, download the binary manually from:"
        echo "  https://github.com/$REPO/releases/latest"
        exit 1
        ;;
esac

BINARY_NAME="qamax-agent-${OS}-${ARCH}"
echo "Detected: ${OS}/${ARCH}"

# Get installation directory
INSTALL_DIR="${HOME}/.qamax-agent"
CONFIG_DIR="${HOME}/.qamax"
echo "Installing to: $INSTALL_DIR"
echo ""

# Create directories
mkdir -p "$INSTALL_DIR"
mkdir -p "$CONFIG_DIR"
chmod 700 "$CONFIG_DIR"

# Determine version to install
if [ -n "$QAMAX_VERSION" ]; then
    VERSION="$QAMAX_VERSION"
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/${VERSION}/${BINARY_NAME}"
    echo "Installing version: $VERSION"
else
    DOWNLOAD_URL="https://github.com/$REPO/releases/latest/download/${BINARY_NAME}"
    echo "Installing latest version..."
fi

# Download binary
echo "Downloading $BINARY_NAME..."
if command -v curl &> /dev/null; then
    HTTP_CODE=$(curl -sL -w "%{http_code}" -o "$INSTALL_DIR/qamax-agent" "$DOWNLOAD_URL")
    if [ "$HTTP_CODE" -ne 200 ]; then
        rm -f "$INSTALL_DIR/qamax-agent"
        echo "Error: Download failed (HTTP $HTTP_CODE)"
        echo "Check available releases at: https://github.com/$REPO/releases"
        exit 1
    fi
elif command -v wget &> /dev/null; then
    if ! wget -q -O "$INSTALL_DIR/qamax-agent" "$DOWNLOAD_URL"; then
        rm -f "$INSTALL_DIR/qamax-agent"
        echo "Error: Download failed"
        echo "Check available releases at: https://github.com/$REPO/releases"
        exit 1
    fi
else
    echo "Error: curl or wget is required"
    exit 1
fi

chmod +x "$INSTALL_DIR/qamax-agent"
echo "Binary installed to: $INSTALL_DIR/qamax-agent"

# Create symlink in /usr/local/bin (requires sudo)
if [ -w /usr/local/bin ]; then
    ln -sf "$INSTALL_DIR/qamax-agent" /usr/local/bin/qamax-agent
    echo "Created symlink: /usr/local/bin/qamax-agent"
else
    echo ""
    echo "To make 'qamax-agent' available globally, run:"
    echo "   sudo ln -sf $INSTALL_DIR/qamax-agent /usr/local/bin/qamax-agent"
fi

echo ""
echo "Installation complete!"
echo ""
echo "Quick start:"
echo "  qamax-agent login                          # Authenticate via browser"
echo "  qamax-agent projects                       # List your projects"
echo "  qamax-agent run --cloud-url https://app.qamax.co  # Start the agent daemon"
echo ""
echo "Run 'qamax-agent help' for all commands."
echo ""
