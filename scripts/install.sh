#!/bin/sh
set -e
REPO="eulerbutcooler/surang"
VERSION="${1:-latest}"
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m); case "$ARCH" in x86_64) ARCH=amd64;; aarch64|arm64) ARCH=arm64;; esac

if [ "$VERSION" = "latest" ]; then
	VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep -o '"tag_name": *"[^"]*"' | cut -d'"' -f4)
fi
URL="https://github.com/$REPO/releases/download/$VERSION/surang-client-$OS-$ARCH"
echo "installing surang-client $VERSION ($OS/$ARCH)..."
curl -fsSL "$URL" -o /tmp/surang-client
chmod +x /tmp/surang-client
mkdir -p ~/.local/bin && mv /tmp/surang-client ~/.local/bin/surang-client
echo "✓ installed to ~/.local/bin/surang-client — make sure it's on your PATH"
