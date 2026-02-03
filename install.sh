#!/bin/bash
# QualityMax Local Agent Installer for macOS/Linux

set -e

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
        echo "For Windows, download the binary manually."
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

# Copy binary
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY_SRC="$SCRIPT_DIR/go/build/$BINARY_NAME"

if [ ! -f "$BINARY_SRC" ]; then
    # Try building from source
    if command -v go &> /dev/null; then
        echo "Pre-built binary not found. Building from source..."
        (cd "$SCRIPT_DIR/go" && go build -ldflags="-s -w" -o "$INSTALL_DIR/qamax-agent" .)
    else
        echo "Error: Pre-built binary not found at $BINARY_SRC"
        echo "Either build with 'make build-all' in local-agent/go/ or install Go to build from source."
        exit 1
    fi
else
    cp "$BINARY_SRC" "$INSTALL_DIR/qamax-agent"
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
