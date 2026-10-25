#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
[ "$(go env GOOS)" = linux ] || { echo 'Run packaging on Linux with native build tools.' >&2; exit 1; }
arch=$(go env GOARCH)
case "$arch" in
    amd64) rpm_arch=x86_64 ;;
    arm64) rpm_arch=aarch64 ;;
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
mkdir -p "$tmp/source" "$tmp/root/usr/bin" "$tmp/root/usr/share" dist
tar -xJf cmd/dualcommander/DualCommander.tar.xz -C "$tmp/source" --strip-components=1
prefix="$tmp/source/usr/local"
[ -d "$prefix" ] || prefix="$tmp/source/usr"
install -m 755 "$prefix/bin/dualcommander" "$tmp/root/usr/bin/dualcommander"
cp -R "$prefix/share/applications" "$prefix/share/pixmaps" "$tmp/root/usr/share/"
install -D -m 644 LICENSE "$tmp/root/usr/share/licenses/dualcommander/LICENSE"
mkdir "$tmp/root/DEBIAN"
sed -e "s/@VERSION@/$native_version/g" -e "s/@RELEASE@/$release/g" \
    -e "s/@ARCH@/$arch/g" -e "s/@GLIBC_VERSION@/$glibc_version/g" \
    packaging/linux/control.in > "$tmp/root/DEBIAN/control"
dpkg-deb --root-owner-group --build "$tmp/root" "dist/dualcommander-linux-$arch.deb"
rm -r "$tmp/root/DEBIAN"
mkdir -p "$tmp/rpm/BUILD" "$tmp/rpm/RPMS" "$tmp/rpm/SOURCES" "$tmp/rpm/SPECS" "$tmp/rpm/SRPMS"
rpmbuild --define "_topdir $tmp/rpm" --define "package_version $native_version" \
    --define "package_release $release" --define "package_arch $rpm_arch" \
    --define "glibc_version $glibc_version" --define "payload_root $tmp/root" \
    --target "$rpm_arch" -bb packaging/linux/dualcommander.spec
cp "$tmp/rpm/RPMS/$rpm_arch/dualcommander-$native_version-$release.$rpm_arch.rpm" "dist/dualcommander-linux-$arch.rpm"