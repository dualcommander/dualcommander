# Release signing

Signing defaults **off**. Unset flags, or any value other than `true`, keep
existing unsigned builds. Pull requests, branch builds, and manual dispatch
never sign. Only pushed `v*` tags can sign.

## Access controls

Before enabling either flag, create GitHub environment `release-signing` with
required reviewers and deployment rules allowing only release tags (`v*`).
Protect release-tag creation and review the tagged workflow before approving.
Store signing secrets only in this environment, not repository-wide secrets.
Unsigned jobs use `unsigned-build`, with no signing credentials.

Set feature flags as **repository Actions variables**, so job environment
selection can read them before environment approval:

| Variable | Default | Enable |
| --- | --- | --- |
| `MACOS_SIGNING_ENABLED` | unset/off | `true` |
| `WINDOWS_SIGNING_ENABLED` | unset/off | `true` |

Enabled signing failures block release publication; there is no unsigned fallback.

## macOS: Developer ID and notarization

Requires Apple Developer Program membership, a Developer ID Application
certificate with private key exported as password-protected PKCS#12, and an
App Store Connect team API key authorized for notarization.

Configure these in `release-signing`:

| Kind | Name | Value |
| --- | --- | --- |
| Secret | `APPLE_CERTIFICATE_BASE64` | Base64-encoded `.p12` |
| Secret | `APPLE_CERTIFICATE_PASSWORD` | PKCS#12 password |
| Secret | `APPLE_API_KEY_BASE64` | Base64-encoded API `.p8` private key |
| Variable | `APPLE_SIGNING_IDENTITY` | Full `Developer ID Application: … (TEAMID)` identity |
| Variable | `APPLE_API_KEY_ID` | API key ID |
| Variable | `APPLE_API_ISSUER_ID` | API issuer ID |

Both macOS architectures sign with hardened runtime and secure timestamps,
notarize and staple the app, then rebuild ZIP/DMG. DMG is also signed,
notarized, stapled, and verified. Temporary keychain and credentials are removed
on exit. No real certificate is required for mocked script tests.

## Windows: SignPath

Enroll with SignPath (for eligible open-source projects, apply to SignPath
Foundation). Configure a project, artifact configuration for the uploaded
`DualCommander.exe`, and a production signing policy with timestamping.
Configure SignPath's GitHub integration and approval policy for this repository
and release workflow; do not allow arbitrary uploaded binaries to be signed.

Configure these in `release-signing`:

| Kind | Name | Value |
| --- | --- | --- |
| Secret | `SIGNPATH_API_TOKEN` | Token authorized to submit signing requests |
| Variable | `SIGNPATH_ORGANIZATION_ID` | Organization ID |
| Variable | `SIGNPATH_PROJECT_SLUG` | Project slug |
| Variable | `SIGNPATH_SIGNING_POLICY_SLUG` | Production signing policy slug |
| Variable | `WINDOWS_SIGNER_SUBJECT` | Exact expected certificate subject |

CI uploads an unsigned executable for signing, waits for approval/completion,
then requires a valid Authenticode signature, expected signer, and timestamp
before creating the public ZIP. Unsigned submission artifacts expire after one
day and are excluded from release assets.

## Enable and verify

Enable one flag at a time after credentials and policies are ready. Push a
reviewed release tag, approve the environment deployment, and complete any
SignPath approval. Verify downloaded packages on real macOS/Windows machines.
Signing supports trusted direct distribution, not automatic store publication;
Windows SmartScreen reputation warnings may still occur.

Run `make check` for mocked Apple success/rejection/failure checks and workflow
wiring checks. These do not replace real notarization or Authenticode validation.