#!/bin/sh
# Run on macOS with ImageMagick installed.
set -eu
cd "$(dirname "$0")/.."
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT

magick -background none assets/dualcommander.svg assets/dualcommander.png
magick assets/dualcommander.png -define icon:auto-resize=256,128,64,48,32,24,16 assets/dualcommander.ico

mkdir "$stage/dualcommander.iconset"
for size in 16 32 128 256 512; do
    magick assets/dualcommander.png -resize "${size}x${size}" "$stage/dualcommander.iconset/icon_${size}x${size}.png"
    double=$((size * 2))
    magick assets/dualcommander.png -resize "${double}x${double}" "$stage/dualcommander.iconset/icon_${size}x${size}@2x.png"
done
iconutil -c icns "$stage/dualcommander.iconset" -o assets/dualcommander.icns