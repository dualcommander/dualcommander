#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
: "${APPLE_API_KEY_BASE64:?Missing API private key}"
: "${APPLE_API_KEY_ID:?Missing API key ID}"
: "${APPLE_API_ISSUER_ID:?Missing API issuer ID}"
: "${APPLE_STORE_APP_ID:?Missing numeric App Store app ID}"
: "${APPLE_STORE_PROVIDER_ID:?Missing provider public ID}"
: "${APPLE_STORE_BUNDLE_ID:?Missing bundle ID}"
: "${RELEASE_TAG:?Missing release tag}"
: "${GITHUB_RUN_NUMBER:?Missing CI build number}"
: "${GITHUB_RUN_ATTEMPT:?Missing CI run attempt}"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
export API_PRIVATE_KEYS_DIR="$work"
umask 077
printf '%s' "$APPLE_API_KEY_BASE64" | base64 --decode > "$work/AuthKey_$APPLE_API_KEY_ID.p8"
pkg=dist/apple-store/dualcommander.pkg
xcrun altool --validate-app -f "$pkg" --type macos --apiKey "$APPLE_API_KEY_ID" --apiIssuer "$APPLE_API_ISSUER_ID"
xcrun altool --upload-package "$pkg" --type macos \
    --asc-public-id "$APPLE_STORE_PROVIDER_ID" --apple-id "$APPLE_STORE_APP_ID" \
    --bundle-version "$GITHUB_RUN_NUMBER.$GITHUB_RUN_ATTEMPT" \
    --bundle-short-version-string "${RELEASE_TAG#v}" --bundle-id "$APPLE_STORE_BUNDLE_ID" \
    --apiKey "$APPLE_API_KEY_ID" --apiIssuer "$APPLE_API_ISSUER_ID"