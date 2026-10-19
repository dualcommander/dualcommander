#!/bin/sh
set -eu
for attempt in 1 2 3; do
    if go install fyne.io/tools/cmd/fyne@v1.7.3; then
        exit 0
    fi
    if [ "$attempt" -eq 3 ]; then
        exit 1
    fi
    printf 'Packager install failed; retrying in %s seconds\n' "$((attempt * 5))" >&2
    sleep "$((attempt * 5))"
done