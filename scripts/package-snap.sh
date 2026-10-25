#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
[ "$(go env GOOS)" = linux ] || { echo 'Run Snap packaging on Linux.' >&2; exit 1; }
arch=$(go env GOARCH)
case "$arch" in
    amd64|arm64) ;;
    *) echo "Unsupported Snap architecture: $arch" >&2; exit 1 ;;
esac
version=$(sh scripts/version.sh)
# Snap versions are at most 32 characters and allow only these characters.
case "$version" in
    ''|*[!a-zA-Z0-9.+:~-]*) echo "Invalid Snap version: $version" >&2; exit 1 ;;
esac
[ "${#version}" -le 32 ] || { echo 'Snap version exceeds 32 characters.' >&2; exit 1; }
root=$(pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/source" "$tmp/payload/bin" "$tmp/payload/meta/gui" "$tmp/snap" "$tmp/output" dist
tar -xJf cmd/dualcommander/DualCommander.tar.xz -C "$tmp/source" --strip-components=1
prefix="$tmp/source/usr/local"
[ -d "$prefix" ] || prefix="$tmp/source/usr"
install -m 755 "$prefix/bin/dualcommander" "$tmp/payload/bin/dualcommander"
cp "$prefix/share/pixmaps/io.github.typekpb.gommander.png" "$tmp/payload/meta/gui/dualcommander.png"
cp LICENSE "$tmp/payload/LICENSE"
cat > "$tmp/payload/meta/gui/dualcommander.desktop" <<'EOF'
[Desktop Entry]
Type=Application
Name=DualCommander
Comment=Two-pane file manager
Exec=dualcommander
Icon=${SNAP}/meta/gui/dualcommander.png
Terminal=false
Categories=System;FileTools;FileManager;
EOF
sed "s/^version: .*/version: '$version'/" snap/snapcraft.yaml > "$tmp/snap/snapcraft.yaml"
(
    cd "$tmp"
    # Intentional word splitting permits Snapcraft flags supplied through Make.
    snapcraft pack ${SNAPCRAFT_ARGS:-} --output "$tmp/output"
)
set -- "$tmp/output/"*.snap
[ "$#" -eq 1 ] && [ -f "$1" ] || { echo 'Expected exactly one Snap output.' >&2; exit 1; }
cp "$1" "$root/dist/dualcommander-linux-$arch.snap"