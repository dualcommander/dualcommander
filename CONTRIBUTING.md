# Contributing to DualCommander

For downloads and usage, see [README.md](README.md).

## Dependabot automation

Enable **Allow auto-merge** and **Allow squash merging** in Settings > General.
Protect `main`: require **Build checks** and **Require branches to be up to date
before merging**, without workflow bypass. Without required checks, auto-merge
can happen before CI finishes.

Dependabot PRs auto-merge after all six builds pass and rebase on each `main` push.
No extra secrets needed. Required reviews still need approval; merge queues are
not supported by this setup.

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
winget install --exact --id 7zip.7zip --source winget
```

Open **MSYS2 MINGW64**, then:

```sh
pacman -Syu  # reopen MINGW64 and repeat if prompted
pacman -S --needed make git python mingw-w64-x86_64-gcc
export PATH="/c/Program Files/Go/bin:/c/Program Files/PowerShell/7:$PATH"
```

Run project commands in MINGW64. For local/unreleased testing without signing:

```sh
make install-packager
make package-native
make archive-windows
```

Extract the entire `dist/dualcommander-windows-amd64.zip` and launch top-level
`DualCommander.exe`. Keep `software/`: startup automatically selects system OpenGL
or bundled Mesa CPU rendering for GPU-less VMs. Plain `make run`/`make build` do
not bundle Mesa. Runtime versions/checksums live in `packaging/windows/runtime.json`;
Mesa updates and license review are manual (Dependabot cannot update this manifest).

For MSIX testing on EC2, use Windows with an interactive RDP desktop. Download an
unreleased branch artifact `dualcommander-windows-store` from GitHub Actions, or
run `make package-msix` with both architecture ZIPs and Store identity environment
values. No release tag needed; `RELEASE_TAG=v0.0.1` supplies local package version.
Install **Windows SDK 10.0.26100.9457**, selecting **Signing Tools for Desktop Apps**,
**Windows SDK for UWP C++ Apps**, and **Windows App Certification Kit**. Tested tools
reside in `C:\Program Files (x86)\Windows Kits\10\bin\10.0.26100.0\x64`.

Extract the downloaded artifact ZIP into the directory below; do not unpack the
`.msixbundle` itself. Close DualCommander and finish file operations first. Run
this block in **administrator PowerShell under the same account used for testing**.
Edit `$directory` if needed. It reads package identity, creates/reuses a local test
certificate, signs a copy, verifies, installs, and requests Start-menu launch.

**Use only on a disposable test machine. This adds machine-wide trust for the
locally generated certificate. Never share its private key or trust an unknown
certificate. Same-version replacement requires uninstalling the old package;
the script asks first because uninstalling can delete settings/app-local data.
Back up needed data before agreeing. Original artifact remains unchanged.**

```powershell
& {
    $ErrorActionPreference = 'Stop'
    $directory = 'C:\Users\Administrator\Documents\dualcommander-windows-store'
    $source = Join-Path $directory 'dualcommander.msixbundle'
    $package = Join-Path $directory 'dualcommander-test.msixbundle'
    $tool = 'C:\Program Files (x86)\Windows Kits\10\bin\10.0.26100.0\x64\signtool.exe'
    foreach ($path in @($source, $tool)) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Missing: $path" }
    }
    if (Get-Process -Name DualCommander -ErrorAction SilentlyContinue) {
        throw 'Close DualCommander before updating. Finish file operations first.'
    }

    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip = [System.IO.Compression.ZipFile]::OpenRead($source)
    try {
        $entry = $zip.GetEntry('AppxMetadata/AppxBundleManifest.xml')
        if (-not $entry) { throw 'Bundle manifest missing.' }
        $reader = [System.IO.StreamReader]::new($entry.Open())
        try { [xml]$manifest = $reader.ReadToEnd() } finally { $reader.Dispose() }
    } finally { $zip.Dispose() }
    $identity = $manifest.Bundle.Identity
    if (-not $identity.Name -or -not $identity.Publisher -or -not $identity.Version) {
        throw 'Invalid bundle identity.'
    }

    $cert = Get-ChildItem Cert:\CurrentUser\My -CodeSigningCert |
        Where-Object {
            $_.FriendlyName -eq 'DualCommander local test' -and
            $_.Subject -eq $identity.Publisher -and $_.HasPrivateKey -and
            $_.NotBefore -le (Get-Date) -and $_.NotAfter -gt (Get-Date)
        } | Sort-Object NotAfter -Descending | Select-Object -First 1
    if (-not $cert) {
        $cert = New-SelfSignedCertificate -Type Custom `
            -Subject $identity.Publisher -FriendlyName 'DualCommander local test' `
            -CertStoreLocation 'Cert:\CurrentUser\My' -KeyUsage DigitalSignature `
            -HashAlgorithm SHA256 -KeyAlgorithm RSA -KeyLength 2048 `
            -NotAfter (Get-Date).AddMonths(3) `
            -TextExtension @('2.5.29.37={text}1.3.6.1.5.5.7.3.3', '2.5.29.19={text}')
    }
    Copy-Item -LiteralPath $source -Destination $package -Force
    & $tool sign /fd SHA256 /s My /sha1 $cert.Thumbprint $package
    if ($LASTEXITCODE -ne 0) { throw 'Signing failed.' }
    $publicCert = Join-Path $directory 'dualcommander-test.cer'
    Export-Certificate -Cert $cert -FilePath $publicCert -Force | Out-Null
    Import-Certificate -FilePath $publicCert `
        -CertStoreLocation 'Cert:\LocalMachine\TrustedPeople' | Out-Null
    Write-Host "Test certificate thumbprint (save for cleanup): $($cert.Thumbprint)"
    & $tool verify /pa /v $package
    if ($LASTEXITCODE -ne 0) { throw 'Verification failed.' }

    $old = Get-AppxPackage -Name $identity.Name
    if ($old -and [version]$old.Version -eq [version]$identity.Version) {
        if ((Read-Host 'Same version installed. Uninstall deletes app-local data. Type REPLACE to continue') -cne 'REPLACE') {
            throw 'Cancelled; installed app unchanged.'
        }
        $old | Remove-AppxPackage
    }
    Add-AppxPackage -Path $package
    $installed = Get-AppxPackage -Name $identity.Name
    if (-not $installed) { throw 'Installed package not found for this user.' }
    $appManifest = Get-AppxPackageManifest -Package $installed.PackageFullName
    $appId = @($appManifest.Package.Applications.Application)[0].Id
    Start-Process explorer.exe -ArgumentList (
        'shell:AppsFolder\{0}!{1}' -f $installed.PackageFamilyName, $appId
    )
    Write-Host 'Launch requested. Confirm visible window; otherwise launch from Start without elevation.'
}
```

If Windows still reports `0x80073CFB`, another user may retain the old package;
do not remove other users' installations automatically. Keep original artifact for
Store submission. Confirm visible window and file operations, not merely a running
process; also test Windows 11 and Certification Kit before submission.

After testing, remove certificate trust and private test key in administrator
PowerShell, replacing `SAVED_THUMBPRINT` with the value printed above. Removing
trust prevents future installs using that certificate:

```powershell
Remove-Item 'Cert:\LocalMachine\TrustedPeople\SAVED_THUMBPRINT'
Remove-Item 'Cert:\CurrentUser\My\SAVED_THUMBPRINT'
```

### Linux

Install development tools and libraries from [Fyne's setup guide](https://docs.fyne.io/started/).

### Commands

Run from the repository root:

```sh
make run       # launch the app
make build     # build into dist/
make fmt       # format Go code
make check     # check formatting, vet, and uncached race tests
```

Use `make fmt-check`, `make vet`, or `make test` for individual checks.
These commands work on the host; Dev Containers are optional.
`make profile-check` separately validates Dev Container configuration syntax;
it does not require Docker or a running container.
On Linux, `make package-snap` packages the existing
`cmd/dualcommander/DualCommander.tar.xz` using Snapcraft (classic confinement).
CI builds it natively on amd64 and arm64; `.snap` files ship with other release assets.
Add regression tests for changes; keep blocking work off the UI thread.
See [development guidelines](.agents/skills/dualcommander-guidelines/SKILL.md).

### Dev Container

Run **Dev Containers: Rebuild and Reopen in Container**. The container includes
Go, Make, Python, a C compiler, Fyne development libraries, and Linux DEB/RPM
packaging tools. From `/workspaces/project`, use `make test`, `make check`,
or `make build`. For native Linux packages:

```sh
make download-deps install-packager install-nfpm
make package-native archive-linux package-linux
```

Go installs tools into the developer user's writable GOPATH, already on PATH.
`GOMEMLIMIT=2GiB` bounds Go's soft memory target for race tests in small Docker
VMs; override this environment variable for larger workloads.
Tests are headless; no host display or Docker socket is mounted. Interactive
`make run` needs a desktop outside this sandbox. Snapcraft requires a separate
Snap-capable build host; macOS/Windows packaging and signing require their native
OS tools. Publishing still requires explicit credentials.

Container creation snapshots selected Cline settings into a private host profile
and seeds a container-local copy. That profile can contain provider credentials;
container processes can read them. Network and workspace writes remain enabled.
Do not commit the profile or mount host home, agent sockets, or live Cline data.

### Coding agents

[AGENTS.md](AGENTS.md) holds shared project rules and links to task-specific
[Agent Skills](https://agentskills.io/) in `.agents/skills/`. Edit these sources,
not agent-specific copies.

- Codex and Cursor read `AGENTS.md` and discover `.agents/skills/`.
- Cline reads `AGENTS.md` and discovers skills through `.claude/skills/`.
  Its existing `.clinerules/hooks/PostToolUse` remains Cline-only.
- Claude Code imports `AGENTS.md` through `CLAUDE.md` and discovers
  `.claude/skills/`.
- Gemini CLI imports `AGENTS.md` through `GEMINI.md` and discovers
  `.agents/skills/`.
- GitHub Copilot uses `.github/copilot-instructions.md` to find shared rules;
  skill discovery depends on the Copilot surface and version.

`.claude/skills` is a relative symlink to `.agents/skills`, not a second copy.
On Windows, enable Git symlink checkout (`core.symlinks=true`) with Developer
Mode or appropriate permissions before cloning. Without symlinks or native
skill support, have the agent read `AGENTS.md` and its linked `SKILL.md` files
directly. Hooks do not transfer between agents; every agent must still run
the required checks. Agent discovery varies by version; verify the five
project skills appear in your agent's skill list, or use the direct-read fallback.

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

Every successful CI build (branches, pull requests, manual runs, and tags) creates
Actions artifact `dualcommander-windows-store`, containing `dualcommander.msixbundle`
for x64/arm64. Tagged GitHub Releases also include this unsigned bundle.
Packaging needs no Store credentials, repository variables, or publishing approval,
so fork pull requests can package too.

Development and prerelease bundles use fixed MSIX version `0.0.1.0`; identify builds
by workflow run and commit. Uninstall an existing package before testing an older
version or a different signing certificate. Uninstalling can remove app data;
use a disposable test VM. Bundles need local test signing before sideloading.

After GitHub Release publication, CI submits one x64/arm64 MSIX bundle for stable
`vMAJOR.MINOR.PATCH` tags only. Prereleases/build metadata stay GitHub-only.
Submission uses the secret names from the
[Microsoft Store GitHub Actions guide](https://learn.microsoft.com/en-us/windows/apps/publish/msstore-dev-cli/github-actions?tabs=msix),
set in GitHub environment `windows-store`: `AZURE_AD_TENANT_ID`, `SELLER_ID`,
`AZURE_AD_APPLICATION_CLIENT_ID`, and `AZURE_AD_APPLICATION_SECRET`. Set the
Partner Center Store ID as variable `WINDOWS_STORE_PRODUCT_ID` there too.

MSIX version is `MAJOR.MINOR.PATCH.0`: components must be at most 65535, version
nonzero, and each update newer. Package structure and Store identity (name,
publisher, publisher display name) live in
[`cmd/dualcommander/AppxManifest.xml`](cmd/dualcommander/AppxManifest.xml); they must
match Partner Center's Product identity. CI substitutes only release version and
architecture.

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
   `.provisionprofile`), and `APPLE_API_PRIVATE_KEY` (full text of the App Store
   Connect `.p8` key authorized to upload builds; not base64).
5. Add environment variables `APPLE_STORE_APPLICATION_IDENTITY` (full Store
   application certificate identity, e.g. `Apple Distribution: … (TEAMID)`),
   `APPLE_STORE_INSTALLER_IDENTITY` (`3rd Party Mac Developer Installer: … (TEAMID)`),
   `APPLE_STORE_BUNDLE_ID`, `APPLE_STORE_TEAM_ID`, `APPLE_API_KEY_ID`, and
   `APPLE_API_ISSUER_ID`.
6. Validate sandboxed workflows on macOS and complete App Review requirements.
   Set **repository variable** `APPLE_STORE_ENABLED=true` last; environment-only
   flags cannot gate this job. Push a new stable tag and approve deployment.

CI imports the Store certificates into a temporary keychain with
`apple-actions/import-codesign-certs`, merges both native macOS ZIP executables into
one universal app, checks bundle metadata and distribution profile, embeds the
profile, and signs app and installer. `apple-actions/upload-testflight-build` then
uploads the PKG through `altool` and does not wait for processing. No notarization
is used for Store distribution. Artifact `dualcommander-apple-store` holds the
signed PKG. Both actions receive signing or upload credentials and are referenced
by version tag, not commit SHA. The certificate action removes its keychain after
the job. Build number uses `GITHUB_RUN_NUMBER.GITHUB_RUN_ATTEMPT` (Apple limits:
9999 runs, 99 attempts). Check App Store Connect processing state before rerunning
failed uploads.

With the certificates in the default keychain search list and
`APPLE_STORE_PROFILE_BASE64` plus the identity variables set in the environment,
`make package-apple-store APPLE_STORE_INPUT=macos-artifacts` packages existing macOS
ZIPs without uploading. `make package-msix` packages Windows without publishing; CI
submits the bundle with the Microsoft Store CLI. `make sign-macos` and
`make verify-windows` expose the same signing verification steps used by CI.
Supply credentials through environment variables, never Make command-line
arguments.

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

## Arch Linux installation

Download `dualcommander-linux-amd64.pkg.tar.zst` (or `-arm64`) from
[GitHub Releases](https://github.com/dualcommander/dualcommander/releases).
The package is unsigned and not in the official repositories or AUR; install
only trusted downloads. It needs manual updates.

```sh
sudo pacman -U ./dualcommander-linux-amd64.pkg.tar.zst
```