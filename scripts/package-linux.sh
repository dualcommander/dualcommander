#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
[ "$(go env GOOS)" = linux ] || { echo 'Run packaging on Linux with native build tools.' >&2; exit 1; }
arch=$(go env GOARCH)
case "$arch" in
    amd64|arm64) ;;
    *) echo "Unsupported package architecture: $arch" >&2; exit 1 ;;
esac
version=$(sh scripts/version.sh)
glibc_version=$(getconf GNU_LIBC_VERSION)
glibc_version=${glibc_version#glibc }
native_version=${version%%[-+]*}
release=1
suffix=${version#"$native_version"}
case "$suffix" in
    -*) release="0.$(printf '%s' "${suffix#?}" | tr -c '[:alnum:].' '.')" ;;
    +*) release="1.$(printf '%s' "${suffix#?}" | tr -c '[:alnum:].' '.')" ;;
esac
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/source" dist
tar -xJf cmd/dualcommander/DualCommander.tar.xz -C "$tmp/source" --strip-components=1
prefix="$tmp/source/usr/local"
[ -d "$prefix" ] || prefix="$tmp/source/usr"
desktop-file-validate packaging/linux/io.github.typekpb.gommander.desktop
export ARCH="$arch" VERSION="$native_version" RELEASE="$release" GLIBC_VERSION="$glibc_version" SOURCE="$prefix"
for format in deb rpm; do
    nfpm package --config packaging/linux/nfpm.yaml --packager "$format" --target "dist/dualcommander-linux-$arch.$format"
done