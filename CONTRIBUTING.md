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
make check     # check formatting, vet, helper scripts, and uncached race tests
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
Signing defaults off. Release tags can opt into Apple signing/notarization and
SignPath Windows signing; see [release signing setup](docs/release-signing.md).