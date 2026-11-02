#!/bin/sh
set -eu
# Retry network setup only, never builds, signing, or publishing.
for delay in 10 20 30 40; do
    if "$@"; then exit 0; fi
    printf 'Command failed; retrying in %s seconds: %s\n' "$delay" "$*" >&2
    sleep "$delay"
done
exec "$@"