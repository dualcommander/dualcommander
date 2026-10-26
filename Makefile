.PHONY: check fmt fmt-check vet test build run package-snap

check: fmt-check vet test

fmt:
	gofmt -w .

fmt-check:
	@files=$$(gofmt -l .) || exit $$?; \
	if [ -n "$$files" ]; then printf '%s\n' "$$files" 'Run make fmt'; exit 1; fi

vet:
	go vet ./...

test:
	python3 .devcontainer/cline-profile.py --self-test
	go test -race -count=1 ./...

build:
	mkdir -p dist
	version=$$(sh scripts/version.sh) && go build -ldflags "-X github.com/dualcommander/dualcommander/internal/commander.buildVersion=$$version" -o dist/dualcommander$(shell go env GOEXE) ./cmd/dualcommander

run:
	version=$$(sh scripts/version.sh) && go run -ldflags "-X github.com/dualcommander/dualcommander/internal/commander.buildVersion=$$version" ./cmd/dualcommander

# Requires Linux, Snapcraft, and the Fyne Linux tarball from the packaging build.
package-snap:
	SNAPCRAFT_ARGS="$(SNAPCRAFT_ARGS)" sh scripts/package-snap.sh