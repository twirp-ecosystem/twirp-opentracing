RETOOL=$(CURDIR)/_tools/bin/retool
PATH := ${PWD}/bin:${PWD}/ENV/bin:${PATH}
.DEFAULT_GOAL := all

all: setup test_all

.PHONY: test test_all test_core generate

# Phony commands:
generate:
	PATH=$(CURDIR)/_tools/bin:$(PATH) GOBIN="${PWD}/bin" go install -v github.com/twitchtv/twirp/protoc-gen-twirp
	$(RETOOL) do go generate ./...

test_all: setup test_core

test_core: generate
	$(RETOOL) do errcheck -blank .
	go test -race $(shell go list ./... | grep -v /vendor/ | grep -v /_tools/)

setup:
	./install_proto.bash
	GOPATH=$(CURDIR)/_tools go install github.com/twitchtv/retool/...
	$(RETOOL) build
