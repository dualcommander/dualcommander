# Contributing to DualCommander

For downloads and usage, see [README.md](README.md).

## Development

Requires Go (see [go.mod](go.mod)), Make, Python 3, Git, and a C compiler.

### macOS

With [Homebrew](https://brew.sh/) installed:

```sh
xcode-select --install  # skip if Command Line Tools or Xcode already installed
brew install go python git
```

macOS supplies Make. Store packaging also requires full Xcode.

### Windows (x64)

In PowerShell:

```powershell
winget install --exact --id GoLang.Go
winget install --exact --id MSYS2.MSYS2
winget install --exact --id Microsoft.PowerShell
```

Open **MSYS2 MINGW64**, then:

```sh
pacman -Syu  # reopen MINGW64 and repeat if prompted
pacman -S --needed make git python mingw-w64-x86_64-gcc
export PATH="/c/Program Files/Go/bin:/c/Program Files/PowerShell/7:$PATH"
```

Run project commands in MINGW64. Store packaging additionally needs Windows SDK.

### Linux

Install development tools and libraries from [Fyne's setup guide](https://docs.fyne.io/started/).

### Commands

Run from the repository root:

```sh
make run       # launch the app
make build     # build into dist/
make fmt       # format Go code
make check     # check formatting, vet, development profile, and uncached race tests
```

Use `make fmt-check`, `make vet`, or `make test` for individual checks.
On Linux, `make package-snap` packages the existing
`cmd/dualcommander/DualCommander.tar.xz` using Snapcraft (classic confinement).
CI builds it natively on amd64 and arm64; `.snap` files ship with other release assets.
Add regression tests for changes; keep blocking work off the UI thread.
See [development guidelines](.clinerules/skills/dualcommander-guidelines/SKILL.md).

## CI and tagging

[GitHub Actions](.github/workflows/build.yml) runs on pushes, pull requests, and
manual dispatch. It vets, tests, and packages Linux, Windows, and macOS apps for
amd64 and arm64. Race detection runs except on Windows arm64, where Go does not
support it. Successful builds upload artifacts.

Local and CI commands share Make targets: `make ci-check` checks native builds
with `TARGET_OS` and `TARGET_ARCH` set; `make package-native` runs Fyne packaging.
Use `make archive-macos package-macos`, `make archive-linux package-linux`, or
`make archive-windows` on the matching OS. `make install-packager` installs the
pinned Fyne tool. Workflows keep runner setup and release gates, not build logic.

Maintainers publish releases by tagging the intended commit with a semantic
version such as `v1.2.3` and pushing that tag:

```sh
git tag v1.2.3
git push origin v1.2.3
```

After all platform builds pass, CI publishes packages as GitHub Release assets.
Tagged builds use the tag version; development builds include the commit hash.
Signing and Microsoft/Mac App Store submission default off; setup follows below.

### Release signing

Only pushed `v*` tags sign. Before enabling signing, create GitHub environment
`release-signing` with required reviewers and `v*`-only deployment rules. Protect
tag creation and review tagged workflows before approval. Keep signing secrets
only in this environment, never repository-wide; `unsigned-build` gets none.
Set repository Actions variables `MACOS_SIGNING_ENABLED=true` and/or
`WINDOWS_SIGNING_ENABLED=true` only after setup. Signing failure blocks release;
there is no unsigned fallback. Branches, pull requests, and manual runs never sign.

Configure these in `release-signing`:

| Platform | Environment secrets | Environment variables |
| --- | --- | --- |
| macOS | `APPLE_CERTIFICATE_BASE64`, `APPLE_CERTIFICATE_PASSWORD`, `APPLE_API_KEY_BASE64` | `APPLE_SIGNING_IDENTITY`, `APPLE_API_KEY_ID`, `APPLE_API_ISSUER_ID` |
| Windows | `SIGNPATH_API_TOKEN` | `SIGNPATH_ORGANIZATION_ID`, `SIGNPATH_PROJECT_SLUG`, `SIGNPATH_SIGNING_POLICY_SLUG`, `WINDOWS_SIGNER_SUBJECT` |

macOS requires Apple Developer membership, a Developer ID Application certificate
with private key exported as password-protected PKCS#12 (base64), and an authorized
App Store Connect `.p8` API key (base64). `APPLE_SIGNING_IDENTITY` is the full
`Developer ID Application: … (TEAMID)` identity. CI signs, notarizes, and staples
app and DMG; temporary credentials and keychain are removed on exit.

Windows requires SignPath enrollment and a project/artifact configuration for
`DualCommander.exe`, with a timestamped production signing policy. Restrict its
GitHub integration to this repository's release workflow, not arbitrary uploads.
`WINDOWS_SIGNER_SUBJECT` must match the certificate subject exactly. CI checks
signature, signer, and timestamp before archiving. SignPath approval may be needed;
SmartScreen reputation warnings can still occur.

Enable one platform at a time. Verify downloaded releases with native macOS/Windows
tools; `make check` does not exercise credential-dependent signing or notarization.

### Microsoft Store

After GitHub Release publication, CI submits one x64/arm64 MSIX bundle for stable
`vMAJOR.MINOR.PATCH` tags only. Prereleases/build metadata stay GitHub-only.
MSIX version is `MAJOR.MINOR.PATCH.0`: components must be at most 65535, version
nonzero, and each update newer. Package structure lives in
[`cmd/dualcommander/AppxManifest.xml`](cmd/dualcommander/AppxManifest.xml);
CI substitutes escaped release version, architecture, and Store identity.

Store signs MSIX; SignPath is independent and optional. Actions artifact
`dualcommander-windows-store` is unsigned, not sideload-ready. Certification and
rollout still control availability. Store failures leave GitHub releases intact;
check submission state before rerunning failed jobs. Validate packaging with Windows
SDK MakeAppx and submission in Partner Center; local application checks cover neither.

### Mac App Store uploads (opt-in)

Repository variable `APPLE_STORE_ENABLED=true` enables a protected `apple-store`
job after GitHub Release publication, for stable `vMAJOR.MINOR.PATCH` tag pushes
only. Pull requests, branches, manual runs, and prerelease tags never upload.
Jobs serialize without cancelling active uploads; release one version at a time
because GitHub can replace older pending jobs. Store failure leaves GitHub
releases intact. Developer ID signing (`MACOS_SIGNING_ENABLED`) is independent.

**Keep this flag off until sandbox compatibility is verified.** Mac App Store
requires App Sandbox. Store packages enable sandbox and user-selected read/write
access, not unrestricted filesystem access. Current folder navigation, persisted
paths, external tools, and Trash workflows are not yet Store-qualified. An
entitlement alone does not grant access: native file/folder authorization and
security-scoped bookmark persistence may need application work before release.
Do not bypass sandbox restrictions with broad temporary exception entitlements.

1. Enroll in Apple Developer, register the existing bundle ID
   `io.github.typekpb.gommander`, and create its macOS app in App Store Connect.
   Complete agreements, listing, privacy policy, ratings, pricing, and availability.
2. Create a Mac App Store distribution provisioning profile for that explicit
   bundle ID. Export Store application and installer certificates **with private
   keys together** as password-protected PKCS#12. Developer ID certificates used
   for direct downloads cannot replace these Store certificates.
3. Create GitHub environment `apple-store` with required reviewers and `v*`-only
   deployment rules. Protect tag creation and review tagged workflows before
   approval. Keep credentials only in this environment, never repository-wide.
4. Add environment secrets `APPLE_STORE_CERTIFICATE_BASE64` (PKCS#12),
   `APPLE_STORE_CERTIFICATE_PASSWORD`, `APPLE_STORE_PROFILE_BASE64` (distribution
   `.provisionprofile`), and `APPLE_API_KEY_BASE64` (App Store Connect `.p8` key
   authorized to upload builds).
5. Add environment variables `APPLE_STORE_APPLICATION_IDENTITY` (full Store
   application certificate identity, e.g. `Apple Distribution: … (TEAMID)`),
   `APPLE_STORE_INSTALLER_IDENTITY` (`3rd Party Mac Developer Installer: … (TEAMID)`),
   `APPLE_STORE_BUNDLE_ID`, `APPLE_STORE_TEAM_ID`, `APPLE_STORE_APP_ID` (numeric app
   ID), `APPLE_STORE_PROVIDER_ID` (provider public ID from `xcrun altool
   --list-providers`), `APPLE_API_KEY_ID`, and `APPLE_API_ISSUER_ID`.
6. Validate sandboxed workflows on macOS and complete App Review requirements.
   Set **repository variable** `APPLE_STORE_ENABLED=true` last; environment-only
   flags cannot gate this job. Push a new stable tag and approve deployment.

CI merges both native macOS ZIP executables into one universal app, checks bundle
metadata and distribution profile, embeds the profile, signs app and installer,
then validates and uploads PKG through `altool`. No notarization is used for Store
distribution. Artifact `dualcommander-apple-store` holds the signed PKG.
Temporary credentials and keychain are removed on exit. Build number uses
`GITHUB_RUN_NUMBER.GITHUB_RUN_ATTEMPT` (Apple limits: 9999 runs, 99 attempts).
Check App Store Connect processing state before rerunning failed uploads.

With the environment above configured, `make package-apple-store
APPLE_STORE_INPUT=macos-artifacts` packages existing macOS ZIPs without uploading.
`make publish-apple-store` validates and uploads that PKG. `make package-msix`
and `make publish-windows-store` likewise keep Windows packaging and publishing
separate. `make sign-macos`, `make verify-windows`, and `make publish-release`
expose the same signing verification and GitHub release steps used by CI.
Publishing targets perform real uploads; supply credentials through environment
variables, never Make command-line arguments.

Upload does **not** submit for App Review or automatically release the app.
Select the processed build and submit/release in App Store Connect, including
first submission. Real certificate/profile signing, Apple server validation,
sandbox behavior, and App Review need separate verification; local application
checks do not validate Store readiness.

## Snap installation

With `snapd` installed, download the `.snap` matching your architecture from
[GitHub Releases](https://github.com/dualcommander/dualcommander/releases).
Classic confinement is **not a sandbox**: it allows normal desktop filesystem and
external-app access. `--dangerous` accepts packages without Snap Store assertions;
install only trusted downloads. These packages need manual updates.

```sh
sudo snap install --dangerous --classic ./dualcommander-linux-amd64.snap
dualcommander
```

For arm64, use `dualcommander-linux-arm64.snap` instead.