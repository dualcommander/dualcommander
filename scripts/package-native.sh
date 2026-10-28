#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
version=$(sh scripts/version.sh)
native_version=${version%%[-+]*}
export GOFLAGS="-ldflags=-X=github.com/dualcommander/dualcommander/internal/commander.buildVersion=$version"
# Ubuntu predefines fortification; Fyne appends its own level 3 definition.
if [ "$(go env GOOS)" = linux ]; then export CGO_CFLAGS="${CGO_CFLAGS:--O2 -g} -U_FORTIFY_SOURCE"; fi
cd cmd/dualcommander
fyne package -os "$(go env GOOS)" -release -app-version "$native_version" --use-raw-icon