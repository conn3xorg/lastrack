VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test release clean

build:
	go build -trimpath -ldflags '$(LDFLAGS)' -o lastrack .

test:
	gofmt -l . | (! grep .)
	go vet ./...
	go test ./...

# Static Linux binaries plus checksums, in dist/.
release:
	mkdir -p dist
	for arch in amd64 arm64; do \
		CGO_ENABLED=0 GOOS=linux GOARCH=$$arch go build -trimpath -ldflags '$(LDFLAGS)' \
			-o dist/lastrack_linux_$$arch . || exit 1; \
	done
	cd dist && shasum -a 256 lastrack_linux_* > SHA256SUMS

clean:
	rm -rf dist lastrack
