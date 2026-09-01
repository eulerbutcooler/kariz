#!/bin/sh
set -e
VERSION="${1:-dev}"
DIST="dist/${VERSION}"
rm -rf "$DIST"
mkdir -p "$DIST"

build() { # build <bin> <os> <arch>
	bin=$1; os=$2; arch=$3; ext=""
	[ "$os" = "windows" ] && ext=".exe"
	out="${bin}-${os}-${arch}${ext}"
	echo "→ $out"
	CGO_ENABLED=0 GOOS=$os GOARCH=$arch \
		go build -trimpath -ldflags "-s -w -X main.version=$VERSION" \
		-o "$DIST/$out" "./cmd/$bin"
}

for arch in amd64 arm64; do
	build surang-server linux "$arch"
done
for os in linux darwin; do
	for arch in amd64 arm64; do
		build surang-client "$os" "$arch"
	done
done
build surang-client windows amd64

(cd "$DIST" && sha256sum * > checksums.txt)
echo "done → $DIST"
