# Contributing to DualCommander

For downloads and usage, see [README.md](README.md).

## Development

Install Go (see [go.mod](go.mod)), Make, a POSIX shell, and a C compiler.
Follow [Fyne's setup guide](https://docs.fyne.io/started/) for platform libraries.
On Windows, use Git Bash with Make installed.

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

Maintainers publish releases by tagging the intended commit with a semantic
version such as `v1.2.3` and pushing that tag:

```sh
git tag v1.2.3
git push origin v1.2.3
```

After all platform builds pass, CI publishes packages as GitHub Release assets.
Tagged builds use the tag version; development builds include the commit hash.
Signing and Microsoft Store submission default off; setup follows below.

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

#### Build first upload locally

On Windows, install Windows SDK (including MakeAppx), PowerShell 7 (`pwsh`),
Python 3 (`python`), and Make. No signing certificate or Store API credentials
are needed. Download the amd64 and arm64 Windows ZIPs from the same GitHub
Actions run or release into `dist/`. Extract GitHub's artifact download wrapper
if present; keep the inner `dualcommander-windows-amd64.zip` and
`dualcommander-windows-arm64.zip` intact.

From Git Bash at the repository root, replace the example values with your
Partner Center **Product identity** values and intended release version:

```sh
export RELEASE_TAG='v1.2.3'
export WINDOWS_STORE_IDENTITY_NAME='YOUR_PACKAGE_IDENTITY_NAME'
export WINDOWS_STORE_PUBLISHER='CN=YOUR_PUBLISHER_ID'
export WINDOWS_STORE_PUBLISHER_DISPLAY_NAME='Your publisher display name'
make package-msix
```

Use `make package-msix WINDOWS_STORE_INPUT=path/to/zips` for another input folder.
Alternatively, from PowerShell with the same environment variables set, run
`pwsh -NoProfile -File scripts/package-windows-store.ps1 -InputDirectory dist`.
Output: `dist/windows-store/dualcommander.msixbundle`, containing x64 and ARM64
packages. Upload this file to the first submission's **Packages** page in
Partner Center. This command only packages; it never publishes or signs.
Use matching source checkout and ZIPs, and a version newer than prior Store
uploads. Keep package display name aligned with your reserved Store name.
MakeAppx and first submission still require validation on Windows/Partner Center.

#### Enable automatic updates

1. Reserve an **MSIX** app in Partner Center. Complete its first submission
   manually, including listing, privacy policy, ratings, pricing, markets, and
   `runFullTrust` approval; the CLI cannot create a first loose-MSIX submission.
2. [Configure Microsoft Entra/Partner Center submission access](https://learn.microsoft.com/en-us/windows/uwp/monetize/create-and-manage-submissions-using-windows-store-services)
   and create a client secret.
3. Create GitHub environment `windows-store` with required reviewers and `v*`-only
   deployments. Add environment secrets `WINDOWS_STORE_TENANT_ID`,
   `WINDOWS_STORE_SELLER_ID`, `WINDOWS_STORE_CLIENT_ID`, `WINDOWS_STORE_CLIENT_SECRET`.
4. Add environment variables `WINDOWS_STORE_PRODUCT_ID`,
   `WINDOWS_STORE_IDENTITY_NAME`, `WINDOWS_STORE_PUBLISHER`, and
   `WINDOWS_STORE_PUBLISHER_DISPLAY_NAME`, matching Partner Center Product identity
   exactly, not the Fyne app ID. Set **repository variable**
   `WINDOWS_STORE_ENABLED=true` last; environment-only flags cannot gate this job.
5. Push a new stable tag and approve deployment. Release one version at a time:
   Store jobs serialize, but GitHub can replace older pending jobs. Do not edit
   pending Partner Center submissions while CI publishes.

Store signs MSIX; SignPath is independent and optional. Actions artifact
`dualcommander-windows-store` is unsigned, not sideload-ready. Certification and
rollout still control availability. Store failures leave GitHub releases intact;
check submission state before rerunning failed jobs. Validate packaging with Windows
SDK MakeAppx and submission in Partner Center; local application checks cover neither.

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