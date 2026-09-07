.PHONY: build test bench clean release-check doctor

VERSION ?= 0.1.3-beta
BIN := bin/bffx

build:
	go build -o $(BIN) ./cmd/bffx

test:
	go test -race -count=1 -timeout=120s ./pkg/... ./tests/api/...

bench:
	go test -bench=. ./pkg/storage/...

doctor: build
	./$(BIN) doctor

clean:
	rm -rf bin/
	rm -rf .bffx/

release-check:
	# Verifies that goreleaser config is valid
	goreleaser check

all: clean build test release-check
