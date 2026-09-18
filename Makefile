VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  = -X main.Version=$(VERSION)

.PHONY: build test cover lint fmt-check tidy smoke eval-t1 eval-t2 eval-adherence package install-routes release-gate clean help

help:             ## list the targets
	@grep -hE '^[a-z0-9-]+:.*##' $(MAKEFILE_LIST) | sed 's/:.*##/\t/' | expand -t22

build:            ## bin/boxer bin/boxer-eval bin/fakellm, all version-stamped
	go build -ldflags "$(LDFLAGS)" -o bin/boxer ./cmd/boxer
	go build -ldflags "$(LDFLAGS)" -o bin/boxer-eval ./cmd/boxer-eval
	go build -ldflags "$(LDFLAGS)" -o bin/fakellm ./cmd/fakellm

test:             ## vet, race detector, coverage floor; fake smolvm, safe to run any time
	go vet ./...
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	./scripts/coverfloor.sh coverage.out

cover: test       ## per-function coverage for the package you are working on: make cover PKG=./internal/box
	go tool cover -func=coverage.out | grep -E '$(or $(PKG),.)' | sort -k3 -n | head -40

lint:             ## golangci-lint with the repository's linter set
	golangci-lint run

fmt-check:        ## fail if anything needs gofmt
	@out=$$(gofmt -l cmd internal pkg); \
	if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

tidy:             ## tidy modules and format in place
	go mod tidy
	gofmt -w cmd internal pkg

smoke: build      ## real smolvm, no model; serialised by the host eval lock
	evals/smoke.sh

eval-t1: build    ## real harness CLIs + scripted model + real smolvm; report to docs/eval-t1.md
	bin/boxer-eval --tier t1 --out docs/eval-t1.md

eval-t2: build    ## live models through the gateway; credentials from .env or evals/.env
	set -a; for f in .env evals/.env; do [ -f "$$f" ] && . "./$$f"; done; set +a; \
	bin/boxer-eval --tier t2 --out docs/eval-t2.md

eval-adherence: build  ## does a live model follow the brief; four models, report to docs/eval-adherence.md
	set -a; for f in .env evals/.env; do [ -f "$$f" ] && . "./$$f"; done; set +a; \
	bin/boxer-eval --tier adherence --out docs/eval-adherence.md

package: build    ## render the published package and every client view into dist/
	bin/boxer package all --out dist

install-routes:   ## install.sh and the npm package against a release staged on this machine
	./scripts/install-routes.sh

release-gate:     ## the mechanical half of the release gate in docs/release.md; no VM, no model
	$(MAKE) fmt-check
	go vet ./...
	go mod tidy -diff
	$(MAKE) test
	golangci-lint run
	govulncheck ./...
	go run ./cmd/boxer package all --out dist/gate
	goreleaser check
	$(MAKE) install-routes

clean:            ## remove build output and coverage
	rm -rf bin dist coverage.out
