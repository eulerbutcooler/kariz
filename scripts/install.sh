#!/bin/sh
set -e
REPO="eulerbutcooler/surang"
VERSION="${1:-latest}"
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in x86_64) ARCH=amd64 ;; aarch64|arm64) ARCH=arm64 ;; esac

if [ "$VERSION" = "latest" ]; then
	VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
		| grep -o '"tag_name": *"[^"]*"' | cut -d'"' -f4)
fi
URL="https://github.com/$REPO/releases/download/$VERSION/surang-client-$OS-$ARCH"
echo "installing surang-client $VERSION ($OS/$ARCH)..."

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
curl -fsSL "$URL" -o "$TMP/surang-client"
chmod +x "$TMP/surang-client"

# install into the first PATH directory under $HOME, else ~/.local/bin
BIN_DIR=""
for d in $(echo "$PATH" | tr ':' ' '); do
	case "$d" in "$HOME"*) BIN_DIR="$d"; break ;; esac
done
[ -z "$BIN_DIR" ] && BIN_DIR="$HOME/.local/bin"
mkdir -p "$BIN_DIR"
mv "$TMP/surang-client" "$BIN_DIR/surang-client"

echo "installed to $BIN_DIR/surang-client"
case ":$PATH:" in
	*":$BIN_DIR:"*) ;;
	*) echo "note: $BIN_DIR is not on your PATH. add it with:"
	   echo "  echo 'export PATH=\"\$PATH:$BIN_DIR\"' >> ~/.bashrc && source ~/.bashrc" ;;
esac
echo "next: surang-client login"
