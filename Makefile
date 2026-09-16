BINARY      := matsrun
CARGO_BIN   := $(HOME)/.cargo/bin
MAIN_PKG    := ./cmd/matsrun
VERSION     ?= 0.1.0
VERSION_PKG := github.com/otterlab-bio/matsrun/pkg/cli
LDFLAGS     := -s -w -X $(VERSION_PKG).Version=$(VERSION)

.PHONY: build install test clean

build:
	CGO_ENABLED=0 go build -buildvcs=false -ldflags="$(LDFLAGS)" -o $(BINARY) $(MAIN_PKG)

install: build
	cp $(BINARY) $(CARGO_BIN)/$(BINARY)
	@echo "Installed to $(CARGO_BIN)/$(BINARY)"

test:
	go test ./...

clean:
	rm -f $(BINARY)
