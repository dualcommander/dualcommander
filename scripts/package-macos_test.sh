#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
dmg="dist/dualcommander-darwin-$(go env GOARCH).dmg"
hdiutil verify "$dmg"
tmp=$(mktemp -d)
mounted=false
cleanup() {
    if [ "$mounted" = true ]; then
        hdiutil detach "$tmp/mount" || return 1
    fi
    rm -rf "$tmp"
}
trap cleanup EXIT
hdiutil attach -readonly -nobrowse -mountpoint "$tmp/mount" "$dmg"
mounted=true
bundle="$tmp/mount/DualCommander.app"
plutil -lint "$bundle/Contents/Info.plist"
exe=$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$bundle/Contents/Info.plist")
[ -x "$bundle/Contents/MacOS/$exe" ]
printf 'macOS DMG checks passed\n'