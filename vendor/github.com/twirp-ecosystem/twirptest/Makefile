TOOLS = $(CURDIR)/_tools/
export GOBIN := $(TOOLS)/bin
export PATH := $(GOBIN):${PATH}
.DEFAULT_GOAL := all

.PHONY: all
all: bootstrap format lint build test

.PHONY: ci
ci: bootstrap lint build test

.PHONY: clean
clean:
	go clean -cache -testcache -modcache
	rm -rf _tools/bin _tools/pkg

.PHONY: bootstrap
bootstrap:
	cd $(TOOLS) && go install github.com/golangci/golangci-lint/cmd/golangci-lint
	cd $(TOOLS) && go install golang.org/x/tools/cmd/goimports
	cd $(TOOLS) && go install github.com/kisielk/errcheck
	cd $(TOOLS) && go install github.com/golang/protobuf/protoc-gen-go
	cd $(TOOLS) && go install github.com/gogo/protobuf/protoc-gen-gofast
	cd $(TOOLS) && go install github.com/twitchtv/twirp/protoc-gen-twirp

.PHONY: generate
generate:
	go generate ./...

.PHONY: format
format: generate
	goimports -l -w .

.PHONY: lint
lint: generate format
	golangci-lint run

.PHONY: build
build:
	go build ./...

.PHONY: test
test: generate
	errcheck -blank .
	go test -race $(shell go list ./... | grep -v /vendor/ | grep -v /_tools/)
