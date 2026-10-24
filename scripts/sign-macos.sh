#!/bin/sh
# Release-only caller supplies credentials through a protected environment.
set -eu
cd "$(dirname "$0")/.."
: "${APPLE_CERTIFICATE_BASE64:?Missing Developer ID Application certificate}"
: "${APPLE_CERTIFICATE_PASSWORD:?Missing certificate password}"
: "${APPLE_SIGNING_IDENTITY:?Missing signing identity}"
: "${APPLE_API_KEY_BASE64:?Missing App Store Connect API private key}"
: "${APPLE_API_KEY_ID:?Missing API key ID}"
: "${APPLE_API_ISSUER_ID:?Missing API issuer ID}"
arch=$(go env GOARCH)
app=cmd/dualcommander/DualCommander.app
work=$(mktemp -d)
keychain="$work/signing.keychain-db"
cleanup() {
    security delete-keychain "$keychain"
    rm -rf "$work"
}
trap cleanup EXIT
password=$(openssl rand -hex 32)
security create-keychain -p "$password" "$keychain"
security set-keychain-settings -lut 21600 "$keychain"
security unlock-keychain -p "$password" "$keychain"
printf '%s' "$APPLE_CERTIFICATE_BASE64" | base64 --decode > "$work/certificate.p12"
printf '%s' "$APPLE_API_KEY_BASE64" | base64 --decode > "$work/key.p8"
chmod 600 "$work/certificate.p12" "$work/key.p8"
security import "$work/certificate.p12" -k "$keychain" -P "$APPLE_CERTIFICATE_PASSWORD" -T /usr/bin/codesign
security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$password" "$keychain"
sign() {
    codesign --force --sign "$APPLE_SIGNING_IDENTITY" --keychain "$keychain" --timestamp --options runtime "$1"
    codesign --verify --strict --verbose=2 "$1"
}
notarize() {
    xcrun notarytool submit "$1" --key "$work/key.p8" --key-id "$APPLE_API_KEY_ID" \
        --issuer "$APPLE_API_ISSUER_ID" --wait --timeout 30m --output-format json > "$work/result.json"
    # notarytool can complete successfully even when Apple rejects the submission.
    python3 -c 'import json,sys; r=json.load(open(sys.argv[1])); print(r); sys.exit(0 if r["status"] == "Accepted" else 1)' "$work/result.json"
}
# Fyne bundle has one native executable and no embedded frameworks.
sign "$app"
mkdir -p dist
app_zip="$work/notarization.zip"
ditto -c -k --sequesterRsrc --keepParent "$app" "$app_zip"
notarize "$app_zip"
xcrun stapler staple "$app"
xcrun stapler validate "$app"
spctl --assess --type execute --verbose=2 "$app"
# Recreate both deliverables from the stapled app, never publish the submission ZIP.
rm -f "dist/dualcommander-darwin-$arch.zip"
ditto -c -k --sequesterRsrc --keepParent "$app" "dist/dualcommander-darwin-$arch.zip"
sh scripts/package-macos.sh
dmg="dist/dualcommander-darwin-$arch.dmg"
sign "$dmg"
notarize "$dmg"
xcrun stapler staple "$dmg"
xcrun stapler validate "$dmg"
spctl --assess --type open --context context:primary-signature --verbose=2 "$dmg"
sh scripts/package-macos_test.sh