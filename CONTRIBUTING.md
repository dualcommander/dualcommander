# Contributing to DualCommander

For downloads and usage, see [README.md](README.md).

## Development setup

Requirements: Go 1.23 or newer (see [go.mod](go.mod)), Make, a POSIX shell,
and a C compiler for cgo. Linux also needs OpenGL development libraries; see
[Fyne's setup guide](https://docs.fyne.io/started/).
On Windows, use Git Bash with Make installed.

Run from the repository root:

```sh
make run   # launch the app
make build # dist/dualcommander (dist/dualcommander.exe on Windows)
make check # formatting check, vet, uncached race tests
```

Without Make, use `go run ./cmd/dualcommander`, `go build ./cmd/dualcommander`,
and `go test -race -count=1 ./...`.

## Cline sandbox (VS Code Dev Containers)

The repository includes a non-root Linux Dev Container with Go and Fyne build
dependencies. Its only configured host bind is this repository, at
`/workspaces/dualcommander`. Build context is `.devcontainer/`, not the repository or
your home directory. No Docker socket, host home, display socket, or extra project
is configured. Linux capabilities are dropped and privilege escalation is blocked;
there is no sudo. Go caches and extension state stay inside container storage.

**A repository configuration cannot force host Cline sessions into a sandbox.**
Use a dedicated VS Code profile and follow these steps before starting Cline:

1. Install Docker and VS Code's **Dev Containers** extension on the host.
   Open this repository root as a single-folder workspace, not its parent.
   Use a standalone clone, not a worktree whose `.git` points outside this folder.
2. In that profile's **host User Settings JSON**, disable credential forwarding:

   ```json
   {
     "dev.containers.copyGitConfig": false,
     "dev.containers.gitCredentialHelperConfigLocation": "none",
     "dev.containers.dockerCredentialHelper": false,
     "dev.containers.githubCLILoginWithToken": false,
     "dev.containers.mountWaylandSocket": false,
     "remote.extensionKind": { "saoudrizwan.claude-dev": ["workspace"] },
     "remote.autoForwardPorts": false
   }
   ```

   Quit that VS Code instance and launch it without `SSH_AUTH_SOCK` in its host
   environment (`env -u SSH_AUTH_SOCK code .` on macOS/Linux). Do not forward GPG
   agents, credentials, extra mounts, or host environment secrets. Do not enable
   host-side Cline/MCP tools, external browser integrations, or remote MCP servers
   with host access. Container settings alone cannot restrict those services.
3. Run **Dev Containers: Reopen in Container**. Start Cline only after VS Code
   shows **Dev Container: DualCommander sandbox**. Verify Cline appears under
   **Dev Container** in the Extensions view, not as a locally running extension.
   Configure only the model API credential needed inside this environment.
4. Run `sh .devcontainer/verify.sh`, then `sh scripts/check.sh` in the container.
   Native macOS icon/golden checks still require macOS CI. No host display is
   shared, so interactive `make run` is not supported by this headless sandbox.

Before trusting the first session, inspect the container from the host:

```sh
docker ps --format 'table {{.ID}}\t{{.Names}}'
# Replace CONTAINER_ID with the matching development container ID.
docker inspect CONTAINER_ID --format '{{json .Mounts}}'
docker inspect CONTAINER_ID --format '{{json .HostConfig.CapDrop}} {{json .HostConfig.SecurityOpt}}'
```

There must be no host bind beyond this repository. Dev Containers may add an
editor-managed `/vscode` volume; reject unexpected credential/socket mounts or
other host paths before starting Cline. Recheck after rebuilding or changing
Dev Containers settings. The startup check verifies process restrictions, not the
host's complete mount configuration.

Network access remains enabled for model APIs and Go downloads; this is not an
egress firewall. Container processes can read/write/delete everything in this
repository, including `.git` and any secrets committed or stored here. Review
changes before executing them on the host. Containers share their runtime's
kernel; use a separate VM for stronger isolation of untrusted code.

## Testing

Add tests for changed behavior. Before submitting a pull request, run:

```sh
sh scripts/check.sh
```

This compatibility wrapper runs `make check`: formatting checks, vet, and race
tests covering file operations, headless UI workflows, and golden PNG renders in
`internal/commander/testdata/`. Check targets never rewrite files or suppress
compiler warnings. Individual targets: `make fmt` (format files), `make fmt-check`,
`make vet`, and `make test`.

After an intended visual change, review `internal/commander/testdata/failed/*.png`,
then update the goldens and rerun checks:

```sh
UPDATE_GOLDEN=1 go test -run TestRenderGolden ./internal/commander
sh scripts/check.sh
```

## Project layout

- `cmd/dualcommander/`: executable entry point and native packaging metadata.
- `internal/commander/`: desktop UI, workflows, tests, and visual fixtures.
- `pkg/`: reusable file operations and platform integration.
- `assets/`: shared app icon and native icon formats.
- `Makefile`: common development commands; `scripts/` keeps compatibility entry points.
- `.github/workflows/`: CI and release builds.

## Native app packages and icons

Install Fyne's packaging tool once:

```sh
go install fyne.io/tools/cmd/fyne@v1.7.3
```

Run on each target OS with its native C compiler and required Fyne development libraries:

```sh
cd cmd/dualcommander
fyne package -os darwin  # DualCommander.app, Finder/Dock icon
fyne package -os windows # DualCommander.exe, embedded Explorer/taskbar icon
fyne package -os linux   # Linux package with desktop launcher and icon
```

`cmd/dualcommander/FyneApp.toml` supplies app name and icon; Fyne generates each
platform's native resources. Ready-to-use icons also live in `assets/`:
`dualcommander.icns` (macOS), `dualcommander.ico` (Windows, 16–256px), and
`dualcommander.png` (Linux/runtime, 1024px).

Plain `go build` sets the window icon but does not create native app packaging.
Versions start at `0.1.0`. `make build`, `make run`, and CI use the exact semantic
version from a release tag (for example, `v0.1.0` becomes `0.1.0`). Non-tag builds
show `0.1.0+<12-character commit SHA>`, with `.dirty` for local changes. Plain Go
builds use embedded VCS metadata, or `0.1.0+unknown` when unavailable; use Make
for tag-aware local builds. About uses this build version, not the native build counter.
Native package version fields use only `major.minor.patch` for Windows/macOS
compatibility; About retains the full version. For manual packaging, export
`GOFLAGS="-ldflags=-X=github.com/dualcommander/dualcommander/internal/commander.buildVersion=$(sh scripts/version.sh)"`
from the repository root before running the packaging commands above.
On macOS, `make run` and plain builds also set the running app's Dock icon using
AppKit and the embedded icon; no `.app` bundle is needed for that runtime icon.
The original app ID remains unchanged to preserve saved settings. The Go module
is `github.com/dualcommander/dualcommander`. Packages are unsigned; release signing
requires platform credentials.

## CI and releases

[GitHub Actions](.github/workflows/build.yml) runs the full check gate on macOS
(matching native icon goldens), then runs headless workflow tests with race
detection on every native build runner and builds Linux amd64, Windows amd64,
and macOS arm64/amd64 packages.

Native font/icon golden comparisons run only on macOS; Windows/Linux still run
workflow assertions, command tests, and file-operation tests.

Every successful build uploads downloadable workflow artifacts. Maintainers can
push a `v*` tag to publish the same archives as GitHub Release assets.
macOS/Windows packages are unsigned and not notarized; OS security prompts still apply.