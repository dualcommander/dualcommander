#!/bin/sh
# Store distribution is separate from Developer ID signing and notarization.
set -eu
cd "$(dirname "$0")/.."
[ "$(uname -s)" = Darwin ] || { echo 'Mac App Store packaging requires macOS.' >&2; exit 1; }
: "${RELEASE_TAG:?Missing stable release tag}"
: "${GITHUB_RUN_NUMBER:?Missing CI build number}"
: "${GITHUB_RUN_ATTEMPT:?Missing CI run attempt}"
: "${APPLE_STORE_CERTIFICATE_BASE64:?Missing application and installer PKCS#12 certificates}"
: "${APPLE_STORE_CERTIFICATE_PASSWORD:?Missing certificate password}"
: "${APPLE_STORE_PROFILE_BASE64:?Missing Mac App Store distribution profile}"
: "${APPLE_STORE_APPLICATION_IDENTITY:?Missing Store application signing identity}"
: "${APPLE_STORE_INSTALLER_IDENTITY:?Missing Store installer signing identity}"
: "${APPLE_STORE_BUNDLE_ID:?Missing registered bundle ID}"
: "${APPLE_STORE_TEAM_ID:?Missing Apple team ID}"
input=${1:-dist}
work=$(mktemp -d)
keychain="$work/store.keychain-db"
cleanup() {
    security delete-keychain "$keychain" >/dev/null 2>&1 || true
    rm -rf "$work"
}
trap cleanup EXIT
umask 077
for arch in amd64 arm64; do
    ditto -x -k "$input/dualcommander-darwin-$arch.zip" "$work/$arch"
done
app="$work/amd64/DualCommander.app"
other="$work/arm64/DualCommander.app"
plist="$app/Contents/Info.plist"
executable=$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$plist")
case "$executable" in
    ''|.|..|*/*) echo 'Invalid bundle executable name.' >&2; exit 1 ;;
esac
[ "$(/usr/libexec/PlistBuddy -c 'Print :CFBundleExecutable' "$other/Contents/Info.plist")" = "$executable" ] || {
    echo 'Architecture bundles have different executable names.' >&2; exit 1;
}
# ponytail: Fyne bundles contain one executable; revisit if frameworks/helpers are added.
xcrun lipo -verify_arch x86_64 "$app/Contents/MacOS/$executable"
xcrun lipo -verify_arch arm64 "$other/Contents/MacOS/$executable"
codesign --remove-signature "$app/Contents/MacOS/$executable"
codesign --remove-signature "$other/Contents/MacOS/$executable"
rm -rf "$app/Contents/_CodeSignature" "$other/Contents/_CodeSignature"
xcrun lipo -create "$app/Contents/MacOS/$executable" "$other/Contents/MacOS/$executable" \
    -output "$work/universal"
mv "$work/universal" "$app/Contents/MacOS/$executable"
chmod 755 "$app/Contents/MacOS/$executable"
xcrun lipo -verify_arch x86_64 arm64 "$app/Contents/MacOS/$executable"
printf '%s' "$APPLE_STORE_PROFILE_BASE64" | base64 --decode > "$app/Contents/embedded.provisionprofile"
security cms -D -i "$app/Contents/embedded.provisionprofile" > "$work/profile.plist"
python3 - "$plist" "$other/Contents/Info.plist" "$work/profile.plist" "$work/entitlements.plist" <<'PY'
import datetime
import os
import plistlib
import re
import sys
from string import Template

tag = os.environ['RELEASE_TAG']
if not re.fullmatch(r'v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)', tag):
    raise ValueError('Store releases require vMAJOR.MINOR.PATCH tags')
version = tag[1:]
build = os.environ['GITHUB_RUN_NUMBER'] + '.' + os.environ['GITHUB_RUN_ATTEMPT']
# ponytail: Apple's build components cap CI run number at 9999; migrate numbering then.
if not re.fullmatch(r'[1-9][0-9]{0,3}\.[1-9][0-9]?', build):
    raise ValueError('Store build requires run number 1..9999 and attempt 1..99')
with open(sys.argv[1], 'rb') as source:
    info = plistlib.load(source)
with open(sys.argv[2], 'rb') as source:
    other = plistlib.load(source)
bundle = os.environ['APPLE_STORE_BUNDLE_ID']
team = os.environ['APPLE_STORE_TEAM_ID']
for candidate in (info, other):
    if candidate['CFBundleIdentifier'] != bundle or candidate['CFBundleShortVersionString'] != version:
        raise ValueError('Both architecture bundles must match Store bundle ID and release version')
with open(sys.argv[3], 'rb') as source:
    profile = plistlib.load(source)
allowed = profile['Entitlements']
identifier = team + '.' + bundle
if (profile['ExpirationDate'] <= datetime.datetime.now(datetime.timezone.utc).replace(tzinfo=None)
        or profile.get('TeamIdentifier') != [team]
        or allowed.get('com.apple.application-identifier') != identifier
        or allowed.get('com.apple.developer.team-identifier') != team
        or allowed.get('get-task-allow', False)
        or 'ProvisionedDevices' in profile or profile.get('ProvisionsAllDevices', False)):
    raise ValueError('Require unexpired Mac App Store distribution profile matching team and bundle ID')
with open('packaging/macos/store-entitlements.plist', 'rb') as source:
    entitlements = plistlib.load(source)
identity = {'APPLE_STORE_TEAM_ID': team, 'APPLE_STORE_BUNDLE_ID': bundle}
for key, value in entitlements.items():
    if isinstance(value, str):
        entitlements[key] = Template(value).substitute(identity)
info['CFBundleVersion'] = build
info['LSApplicationCategoryType'] = 'public.app-category.utilities'
with open(sys.argv[1], 'wb') as target:
    plistlib.dump(info, target)
with open(sys.argv[4], 'wb') as target:
    plistlib.dump(entitlements, target)
PY
plutil -lint "$plist" "$work/entitlements.plist"
password=$(openssl rand -hex 32)
security create-keychain -p "$password" "$keychain"
security set-keychain-settings -lut 21600 "$keychain"
security unlock-keychain -p "$password" "$keychain"
printf '%s' "$APPLE_STORE_CERTIFICATE_BASE64" | base64 --decode > "$work/certificates.p12"
security import "$work/certificates.p12" -k "$keychain" -P "$APPLE_STORE_CERTIFICATE_PASSWORD" \
    -T /usr/bin/codesign -T /usr/bin/productbuild
security set-key-partition-list -S apple-tool:,apple:,codesign: -s -k "$password" "$keychain"
codesign --force --sign "$APPLE_STORE_APPLICATION_IDENTITY" --keychain "$keychain" \
    --timestamp --entitlements "$work/entitlements.plist" "$app"
codesign --verify --strict --verbose=2 "$app"
mkdir -p dist/apple-store
productbuild --component "$app" /Applications --sign "$APPLE_STORE_INSTALLER_IDENTITY" \
    --keychain "$keychain" "$work/dualcommander.pkg"
pkgutil --check-signature "$work/dualcommander.pkg"
cp "$work/dualcommander.pkg" dist/apple-store/dualcommander.pkg