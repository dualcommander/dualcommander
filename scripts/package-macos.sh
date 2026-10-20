#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
[ "$(uname -s)" = Darwin ] || { echo 'Run DMG packaging on macOS.' >&2; exit 1; }
arch=$(go env GOARCH)
app=cmd/dualcommander/DualCommander.app
[ -d "$app" ] || { echo 'Build DualCommander.app with fyne package -os darwin first.' >&2; exit 1; }
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT
ditto "$app" "$stage/DualCommander.app"
ln -s /Applications "$stage/Applications"
mkdir -p dist
hdiutil create -ov -volname DualCommander -srcfolder "$stage" -format UDZO \
    "dist/dualcommander-darwin-$arch.dmg"