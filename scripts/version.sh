#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
base=$(sed -n 's/^Version = "\([^"]*\)"/\1/p' cmd/dualcommander/FyneApp.toml)
# SemVer 2.0: numeric prerelease identifiers cannot have leading zeroes.
number='(0|[1-9][0-9]*)'
identifier="($number|[0-9]*[A-Za-z-][0-9A-Za-z-]*)"
semver="$number\.$number\.$number(-$identifier(\.$identifier)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?"
ref=${GITHUB_REF:-}
sha=$(git rev-parse --short=12 HEAD)
dirty=$(git status --porcelain)
tag=""
case "$ref" in
    refs/tags/*) tag=${ref#refs/tags/} ;;
    "")
        if [ -z "$dirty" ]; then
            tag=$(git tag --points-at HEAD | grep -E "^v?$semver$" | sort | head -n 1)
        fi
        ;;
esac
if [ -n "$tag" ]; then
    version=${tag#v}
    if ! printf '%s\n' "$version" | grep -Eq "^$semver$"; then
        printf 'Invalid semantic version tag: %s\n' "$tag" >&2
        exit 1
    fi
    printf '%s\n' "$version"
else
    suffix=""
    [ -z "$dirty" ] || suffix=".dirty"
    printf '%s+%s%s\n' "$base" "$sha" "$suffix"
fi