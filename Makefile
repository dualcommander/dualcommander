.PHONY: check fmt fmt-check vet test profile-check ci-check build run download-deps install-packager install-snapcraft package-native archive-macos archive-linux archive-windows package-macos package-linux sign-macos verify-windows package-snap package-msix package-apple-store publish-apple-store

WINDOWS_STORE_INPUT ?= dist
APPLE_STORE_INPUT ?= dist
# Headless Fyne race tests can exceed Go's 10-minute default on CI runners.
TEST_TIMEOUT ?= 15m

check: fmt-check vet test

fmt:
	gofmt -w .

fmt-check:
	@files=$$(gofmt -l .) || exit $$?; \
	if [ -n "$$files" ]; then printf '%s\n' "$$files" 'Run make fmt'; exit 1; fi

vet:
	go vet ./...

profile-check:
	sh -n .devcontainer/cline-settings.sh
	python3 -m json.tool .devcontainer/devcontainer.json >/dev/null

test:
	go test -race -timeout=$(TEST_TIMEOUT) -count=1 ./...

ci-check:
	test "$$(go env GOOS)" = "$$TARGET_OS"
	test "$$(go env GOARCH)" = "$$TARGET_ARCH"
	go vet -tags ci ./...
	# Go's race detector does not support windows/arm64.
	@flags=-race; \
	if [ "$$TARGET_OS/$$TARGET_ARCH" = windows/arm64 ]; then flags=; fi; \
	go test -tags ci $$flags -timeout=$(TEST_TIMEOUT) -count=1 ./...

build:
	mkdir -p dist
	version=$$(sh scripts/version.sh) && go build -ldflags "-X github.com/dualcommander/dualcommander/internal/commander.buildVersion=$$version" -o dist/dualcommander$(shell go env GOEXE) ./cmd/dualcommander

run:
	version=$$(sh scripts/version.sh) && go run -ldflags "-X github.com/dualcommander/dualcommander/internal/commander.buildVersion=$$version" ./cmd/dualcommander

download-deps:
	go mod download

install-packager:
	go install fyne.io/tools/cmd/fyne@v1.7.4

install-snapcraft:
	sudo snap install snapcraft --classic

package-native:
	sh scripts/package-native.sh

archive-macos:
	mkdir -p dist
	ditto -c -k --sequesterRsrc --keepParent cmd/dualcommander/DualCommander.app "dist/dualcommander-darwin-$$(go env GOARCH).zip"

archive-linux:
	mkdir -p dist
	cp cmd/dualcommander/DualCommander.tar.xz "dist/dualcommander-linux-$$(go env GOARCH).tar.xz"

# Requires 7-Zip and internet access for pinned software OpenGL runtime.
archive-windows:
	pwsh -NoProfile -File scripts/archive-windows.ps1

package-macos:
	sh scripts/package-macos.sh

package-linux:
	sh scripts/package-linux.sh

sign-macos:
	sh scripts/sign-macos.sh

verify-windows:
	pwsh -NoProfile -File scripts/verify-windows.ps1

# Requires Linux, Snapcraft, and the Fyne Linux tarball from the packaging build.
package-snap:
	SNAPCRAFT_ARGS="$(SNAPCRAFT_ARGS)" sh scripts/package-snap.sh

# Requires Windows SDK, PowerShell 7, and both Windows ZIPs.
package-msix:
	pwsh -NoProfile -File scripts/package-windows-store.ps1 -InputDirectory "$(WINDOWS_STORE_INPUT)"

package-apple-store:
	sh scripts/package-apple-store.sh "$(APPLE_STORE_INPUT)"

publish-apple-store:
	sh scripts/publish-apple-store.sh
