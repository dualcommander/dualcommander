#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
arch=$(go env GOARCH)
case "$arch" in
    amd64) rpm_arch=x86_64 ;;
    arm64) rpm_arch=aarch64 ;;
    *) echo "Unsupported package architecture: $arch" >&2; exit 1 ;;
esac
deb=dist/dualcommander-linux-$arch.deb
rpm=dist/dualcommander-linux-$arch.rpm
[ "$(dpkg-deb -f "$deb" Architecture)" = "$arch" ]
[ "$(rpm -qp --qf '%{ARCH}' "$rpm")" = "$rpm_arch" ]
[ "$(dpkg-deb -f "$deb" Version)" = "$(rpm -qp --qf '%{VERSION}-%{RELEASE}' "$rpm")" ]
glibc_version=$(getconf GNU_LIBC_VERSION)
glibc_version=${glibc_version#glibc }
dpkg-deb -f "$deb" Depends | grep -F "libc6 (>= $glibc_version)"
rpm -qp --requires "$rpm" | grep -Fx "glibc >= $glibc_version"
# rpmbuild must retain automatic ELF dependencies, not just hand-written ones.
rpm -qp --requires "$rpm" | grep -F 'libGL.so.1()(64bit)'
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
dpkg-deb -x "$deb" "$tmp/deb"
mkdir "$tmp/rpm"
rpm2cpio "$rpm" | (cd "$tmp/rpm" && cpio -id --quiet)
for format in deb rpm; do
    root="$tmp/$format"
    desktop-file-validate "$root"/usr/share/applications/*.desktop
done
printf 'Linux package checks passed\n'