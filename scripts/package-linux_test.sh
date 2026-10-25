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
[ "$(dpkg-deb -f "$deb" Package)" = dualcommander ]
[ "$(dpkg-deb -f "$deb" Architecture)" = "$arch" ]
[ "$(rpm -qp --qf '%{NAME}' "$rpm")" = dualcommander ]
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
mkdir "$tmp/source"
tar -xJf cmd/dualcommander/DualCommander.tar.xz -C "$tmp/source" --strip-components=1
prefix="$tmp/source/usr/local"
[ -d "$prefix" ] || prefix="$tmp/source/usr"
dpkg-deb -x "$deb" "$tmp/deb"
mkdir "$tmp/rpm"
rpm2cpio "$rpm" | (cd "$tmp/rpm" && cpio -id --quiet)
for format in deb rpm; do
    root="$tmp/$format"
    cmp "$prefix/bin/dualcommander" "$root/usr/bin/dualcommander"
    [ -x "$root/usr/bin/dualcommander" ]
    desktop="$root/usr/share/applications/io.github.typekpb.gommander.desktop"
    grep -Fx 'Exec=dualcommander' "$desktop"
    grep -Fx 'Icon=io.github.typekpb.gommander' "$desktop"
    desktop-file-validate "$desktop"
    [ -s "$root/usr/share/pixmaps/io.github.typekpb.gommander.png" ]
    cmp LICENSE "$root/usr/share/licenses/dualcommander/LICENSE"
    [ ! -e "$root/usr/local" ]
done
printf 'Linux package checks passed\n'