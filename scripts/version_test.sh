#!/bin/sh
set -eu
root=$(CDPATH= cd "$(dirname "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/scripts" "$tmp/cmd/dualcommander"
cp "$root/scripts/version.sh" "$tmp/scripts/"
cp "$root/cmd/dualcommander/FyneApp.toml" "$tmp/cmd/dualcommander/"
cd "$tmp"
git init -q
git add .
git -c core.hooksPath=/dev/null -c user.name=Test -c user.email=test@example.invalid commit -qm initial
sha=$(git rev-parse --short=12 HEAD)
check() {
    actual=$(GITHUB_REF="$1" sh scripts/version.sh)
    if [ "$actual" != "$2" ]; then
        printf 'Expected %s, got %s\n' "$2" "$actual" >&2
        exit 1
    fi
}
check "" "0.1.0+$sha"
git tag v0.1.0
check "" "0.1.0"
check refs/heads/main "0.1.0+$sha"
check refs/tags/v1.2.3 "1.2.3"
check refs/tags/v1.2.3-rc.1+build.2 "1.2.3-rc.1+build.2"
check refs/tags/1.2.3 "1.2.3"
for tag in v01.2.3 v1.2 v1.2.3-01 vnope; do
    if GITHUB_REF="refs/tags/$tag" sh scripts/version.sh >/dev/null 2>&1; then
        printf 'Accepted invalid tag: %s\n' "$tag" >&2
        exit 1
    fi
done
touch local-change
check "" "0.1.0+$sha.dirty"
printf 'Version checks passed\n'