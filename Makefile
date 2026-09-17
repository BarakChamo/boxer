VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  = -X main.Version=$(VERSION)

.PHONY: build test smoke eval-t1 eval-t2 package tidy

build:            ## bin/boxer bin/boxer-eval bin/fakellm
	go build -ldflags "$(LDFLAGS)" -o bin/boxer ./cmd/boxer
	go build -o bin/boxer-eval ./cmd/boxer-eval
	go build -o bin/fakellm ./cmd/fakellm

test:             ## unit tests with the fake smolvm (safe to run in parallel with anything)
	go vet ./... && go test ./...

smoke: build      ## real smolvm, no model (44 checks); serialised by the host eval lock
	evals/smoke.sh

eval-t1: build    ## real harness CLIs + fake model + real smolvm; report to docs/eval-t1.md
	bin/boxer-eval --tier t1 --out docs/eval-t1.md

eval-t2: build    ## live models; credentials from evals/.env (gitignored)
	set -a; for f in .env evals/.env; do [ -f $f ] && . $f; done; set +a; bin/boxer-eval --tier t2 --out docs/eval-t2.md

package: build    ## render every harness bundle into dist/
	bin/boxer package all --out dist

tidy:
	go mod tidy && gofmt -w cmd internal pkg 2>/dev/null || gofmt -w cmd internal
