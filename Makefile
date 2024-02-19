DOCKER := $(shell which docker)
DOCKER_BUF := $(DOCKER) run --rm -v $(CURDIR):/workspace --workdir /workspace bufbuild/buf:1.28.1
PROJECTNAME=$(shell basename "$(PWD)")

## help: Get more info on make commands.
help: Makefile
	@echo " Choose a command run in "$(PROJECTNAME)":"
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'
.PHONY: help

## proto-gen: Generate protobuf files. Requires docker.
proto-gen:
	@echo "--> Generating Protobuf files"
	$(DOCKER_BUF) generate
.PHONY: proto-gen-docker

## proto-lint: Lint protobuf files. Requires docker.
proto-lint:
	@echo "--> Linting Protobuf files"
	@$(DOCKER_BUF) lint
.PHONY: proto-lint

## lint: Lint Go files. Requires golangci-lint.
lint:
	@echo "--> Lint source code using golangci-lint"
	@golangci-lint run
.PHONY: lint

## test: Run unit tests.
test:
	@echo "--> Run unit tests"
	@go test -mod=readonly ./...
.PHONY: test

## benchmark: Run tests in benchmark mode.
benchmark:
	@echo "--> Perform benchmark"
	@go test -mod=readonly -bench=. ./...
.PHONY: benchmark
